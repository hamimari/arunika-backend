package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrCartChanged is returned by CreateOrder when the parent must look at the
// cart again; CartChangedError carries the new cart.
var ErrCartChanged = errors.New("CART_CHANGED")

type CartChangedError struct{ Cart *CartView }

func (e *CartChangedError) Error() string { return ErrCartChanged.Error() }
func (e *CartChangedError) Unwrap() error { return ErrCartChanged }

// CheckoutOrderView is what the app needs to launch the store purchase.
type CheckoutOrderView struct {
	OrderID          uuid.UUID          `json:"order_id"`
	PlayProductID    string             `json:"play_product_id"`
	SubtotalIdr      int64              `json:"subtotal_idr"`
	PromoSavingIdr   int64              `json:"promo_saving_idr"`
	TotalIdr         int64              `json:"total_idr"`
	ChargeIdr        int64              `json:"charge_idr"`
	PriceLockedUntil time.Time          `json:"price_locked_until"`
	Items            []models.OrderItem `json:"items"`
}

// StoreProductFor picks the store product charged for total. Google Play
// has one product per Rp 1.000, so the match is exact.
func StoreProductFor(db *gorm.DB, store string, total int64) (*models.StoreProduct, error) {
	var sp models.StoreProduct
	err := db.Where("store = ? AND price_idr = ? AND active", store, total).First(&sp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCartAmountUnavailable
	}
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

// StoreProductSKU is the store product id for a total: arunika.cart.t106000.
func StoreProductSKU(totalIdr int64) string {
	return fmt.Sprintf("arunika.cart.t%d", totalIdr)
}

// CreateOrder freezes the cart (productIDs nil) or an explicit item list
// (a single "Beli") into a PENDING order with locked prices. For a cart it
// first re-checks every item and returns *CartChangedError instead of an
// order when anything other than a price drop moved. A repeated
// idempotencyKey returns the order created the first time.
func (s *CartService) CreateOrder(userID uuid.UUID, productIDs []uuid.UUID, idempotencyKey string) (*CheckoutOrderView, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey != "" {
		var existing models.Order
		err := s.db.Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).First(&existing).Error
		if err == nil {
			return s.checkoutView(s.db, &existing)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	// Re-check the cart outside the order transaction: the clean-up of
	// owned/free items and the accepted prices must persist even though no
	// order is created.
	if productIDs == nil {
		if err := s.silentlyAcceptDrops(s.db, userID); err != nil {
			return nil, err
		}
		cart, err := s.read(s.db, userID)
		if err != nil {
			return nil, err
		}
		if cart.HasChanges() {
			// The parent sees the changes once; the next Bayar accepts them.
			if err := s.acceptPrices(s.db, userID); err != nil {
				return nil, err
			}
			return nil, &CartChangedError{Cart: cart}
		}
	}

	var result *CheckoutOrderView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "cart:"+userID.String()).Error; err != nil {
			return err
		}
		if sub, err := s.entitlements.HasActiveSubscription(userID); err != nil {
			return err
		} else if sub {
			return ErrCartSubscription
		}

		var items []CartItemView
		if productIDs == nil {
			cart, err := s.read(tx, userID)
			if err != nil {
				return err
			}
			if cart.HasChanges() {
				// Changed between the check above and the lock.
				return &CartChangedError{Cart: cart}
			}
			items = cart.payable()
		} else {
			var err error
			if items, err = s.directItems(tx, userID, productIDs); err != nil {
				return err
			}
		}
		if len(items) == 0 {
			return ErrCartEmpty
		}
		if len(items) > CartMaxItems {
			return ErrCartFull
		}

		view := CartView{Items: items}
		view.totals()
		if view.TotalIdr > CartMaxTotalIdr {
			return ErrCartTotalLimit
		}
		sp, err := StoreProductFor(tx, models.StoreGoogle, view.TotalIdr)
		if err != nil {
			return err
		}

		now := s.now()
		locked := now.Add(CartPriceLock)
		subtotal, charge := view.SubtotalIdr, sp.PriceIdr
		order := models.Order{
			UserID:           userID,
			AmountIdr:        view.TotalIdr,
			Status:           models.OrderStatusPending,
			Provider:         models.OrderProviderGooglePlay,
			IsCart:           true,
			SubtotalIdr:      &subtotal,
			ChargeIdr:        &charge,
			PriceLockedUntil: &locked,
			StoreProductID:   &sp.ID,
		}
		if idempotencyKey != "" {
			order.IdempotencyKey = &idempotencyKey
		}
		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("create order: %w", err)
		}
		for _, it := range items {
			normal := it.PriceIdr
			if it.StrikePriceIdr != nil && *it.StrikePriceIdr > normal {
				normal = *it.StrikePriceIdr
			}
			if err := tx.Create(&models.OrderItem{
				OrderID: order.ID, ProductID: it.ProductID, Title: it.Title,
				ItemType: it.ItemType, NormalIdr: normal, PriceIdr: it.PriceIdr,
			}).Error; err != nil {
				return fmt.Errorf("create order item: %w", err)
			}
		}
		if err := tx.Create(&models.Payment{
			OrderID:           order.ID,
			ProviderOrderID:   fmt.Sprintf("order-%s", order.ID.String()),
			UserID:            &userID,
			TransactionStatus: "pending",
			PaymentType:       "google_play",
			GrossAmount:       fmt.Sprintf("%d.00", charge),
		}).Error; err != nil {
			return fmt.Errorf("create payment: %w", err)
		}
		var err2 error
		result, err2 = s.checkoutView(tx, &order)
		return err2
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// directItems validates an explicit item list (single "Beli") with the same
// rules as adding to the cart.
func (s *CartService) directItems(tx *gorm.DB, userID uuid.UUID, productIDs []uuid.UUID) ([]CartItemView, error) {
	rules, err := s.strike.LoadRules()
	if err != nil {
		return nil, err
	}
	seen := map[uuid.UUID]bool{}
	var out []CartItemView
	for _, id := range productIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		it, err := s.checkAddable(tx, userID, id)
		if err != nil {
			return nil, err
		}
		scope := models.StrikeScopeArCard
		if it.ItemType == models.ItemTypeDongeng {
			scope = models.StrikeScopeDongeng
		}
		strike := rules.Resolve(scope, it.PriceIdr, it.StrikeOverride, s.now())
		out = append(out, CartItemView{
			ProductID: id, ItemType: it.ItemType, ContentID: it.ContentID, Title: it.Title,
			ImageURL: it.ImageURL, PriceIdr: it.PriceIdr, StrikePriceIdr: strike.StrikePriceIdr,
		})
	}
	return out, nil
}

func (s *CartService) checkoutView(db *gorm.DB, order *models.Order) (*CheckoutOrderView, error) {
	if !order.IsCart || order.StoreProductID == nil {
		return nil, errors.New("not a cart order")
	}
	var sp models.StoreProduct
	if err := db.Where("id = ?", *order.StoreProductID).First(&sp).Error; err != nil {
		return nil, err
	}
	var items []models.OrderItem
	if err := db.Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
		return nil, err
	}
	v := &CheckoutOrderView{
		OrderID:       order.ID,
		PlayProductID: sp.PlayProductID,
		TotalIdr:      order.AmountIdr,
		Items:         items,
	}
	if order.SubtotalIdr != nil {
		v.SubtotalIdr = *order.SubtotalIdr
		v.PromoSavingIdr = *order.SubtotalIdr - order.AmountIdr
	}
	if order.ChargeIdr != nil {
		v.ChargeIdr = *order.ChargeIdr
	}
	if order.PriceLockedUntil != nil {
		v.PriceLockedUntil = *order.PriceLockedUntil
	}
	return v, nil
}

// Order phases shown to the app and the backoffice.
const (
	PhaseWaiting    = "menunggu"
	PhaseProcessing = "diproses"
	PhaseGranted    = "diberikan"
	PhaseFailed     = "gagal"
	PhaseExpired    = "kedaluwarsa"
	PhaseRefunded   = "dikembalikan"
)

// OrderPhase derives the phase from an order: a PENDING order whose
// purchase token is on file has been paid in the store and is waiting for
// its grant.
func OrderPhase(o *models.Order) string {
	switch o.Status {
	case models.OrderStatusPaid:
		return PhaseGranted
	case models.OrderStatusFailed:
		return PhaseFailed
	case models.OrderStatusExpired:
		return PhaseExpired
	case models.OrderStatusRefunded:
		return PhaseRefunded
	}
	if o.PurchaseToken != nil && *o.PurchaseToken != "" {
		return PhaseProcessing
	}
	return PhaseWaiting
}

// OwnedProducts lists the user's active single-item entitlements ("Pulihkan
// pembelian" reads ownership from here, not from the store).
func (s *CartService) OwnedProducts(userID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := s.db.Model(&models.UserEntitlement{}).
		Where("user_id = ? AND (expires_at IS NULL OR expires_at > NOW())", userID).
		Order("created_at").
		Pluck("product_id", &ids).Error
	return ids, err
}
