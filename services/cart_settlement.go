package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"arunika_backend/models"

	"github.com/google/uuid"
)

// Google Play RTDN oneTimeProductNotification types.
const (
	playNotifOneTimePurchased = 1
	playNotifOneTimeCanceled  = 2
)

type playOneTimeProductNotification struct {
	NotificationType int    `json:"notificationType"`
	PurchaseToken    string `json:"purchaseToken"`
	SKU              string `json:"sku"`
}

// CartGrantAlertAfter is how long a paid cart order may wait for its grant
// before the team is alerted.
const CartGrantAlertAfter = 10 * time.Minute

// handleOneTimeRTDN settles a cart order when Google reports its purchase
// completed, so a parent whose app closed mid-payment still gets the items.
// Notifications for anything that is not a cart store product are ignored.
func (s *PaymentService) handleOneTimeRTDN(ctx context.Context, n *playOneTimeProductNotification) error {
	if n.NotificationType != playNotifOneTimePurchased || n.PurchaseToken == "" {
		return nil
	}
	var sp models.StoreProduct
	if err := s.db.Where("play_product_id = ?", n.SKU).First(&sp).Error; err != nil {
		return nil
	}
	purchase, err := s.playVerifier.VerifyProductPurchase(ctx, n.SKU, n.PurchaseToken)
	if err != nil {
		return err
	}
	orderID, err := uuid.Parse(purchase.ObfuscatedExternalAccountID)
	if err != nil {
		slog.Error("cart RTDN: purchase carries no order id", "sku", n.SKU)
		return nil
	}
	order, err := models.FindOrderByID(s.db, orderID)
	if err != nil || !order.IsCart {
		return nil
	}
	_, err = s.VerifyPlayPurchase(ctx, order.ID, order.UserID, n.PurchaseToken)
	if errors.Is(err, ErrPlayPurchasePending) {
		return nil
	}
	return err
}

// RunCartSettlement is one pass of the cart worker: it retries the grant of
// cart orders that were paid in the store but not yet granted, retries the
// server-side consume of granted ones, and alerts on any order waiting for
// its grant longer than CartGrantAlertAfter.
func (s *PaymentService) RunCartSettlement(ctx context.Context) {
	var unsettled []models.Order
	if err := s.db.Where("is_cart AND status = ? AND purchase_token IS NOT NULL AND created_at < ?",
		models.OrderStatusPending, time.Now().Add(-time.Minute)).
		Order("created_at").Limit(100).Find(&unsettled).Error; err != nil {
		slog.Error("cart worker: list unsettled orders", "error", err)
		return
	}
	for i := range unsettled {
		o := &unsettled[i]
		_, err := s.VerifyPlayPurchase(ctx, o.ID, o.UserID, *o.PurchaseToken)
		if err == nil {
			slog.Info("cart worker: order granted", "order_id", o.ID)
			continue
		}
		if errors.Is(err, ErrPlayPurchaseNotValid) || errors.Is(err, ErrPlayPurchaseMismatch) || errors.Is(err, ErrPlayPurchaseTokenReused) {
			// Google says it was never paid (or it is not this order's
			// purchase): close it so it stops being retried.
			s.db.Model(o).Update("status", models.OrderStatusFailed)
			if errors.Is(err, ErrPlayPurchaseTokenReused) {
				slog.Error("ALERT cart purchase token reused", "order_id", o.ID)
			}
			continue
		}
		if time.Since(o.CreatedAt) > CartGrantAlertAfter && !errors.Is(err, ErrPlayPurchasePending) {
			slog.Error("ALERT cart order paid but not granted", "order_id", o.ID, "age", time.Since(o.CreatedAt).Round(time.Second), "error", err)
		}
	}

	var unconsumed []models.Order
	if err := s.db.Where("is_cart AND status = ? AND granted_at IS NOT NULL AND consumed_at IS NULL AND purchase_token IS NOT NULL",
		models.OrderStatusPaid).Order("granted_at").Limit(100).Find(&unconsumed).Error; err != nil {
		slog.Error("cart worker: list unconsumed orders", "error", err)
		return
	}
	for i := range unconsumed {
		s.consumeCartOrder(ctx, &unconsumed[i], *unconsumed[i].PurchaseToken)
	}
}

// ExpireCartOrders marks cart orders never paid (no purchase token) as
// EXPIRED once their price lock has long passed. Their cart is untouched.
func (s *PaymentService) ExpireCartOrders() (int64, error) {
	res := s.db.Model(&models.Order{}).
		Where("is_cart AND status = ? AND purchase_token IS NULL AND price_locked_until < ?",
			models.OrderStatusPending, time.Now().Add(-24*time.Hour)).
		Update("status", models.OrderStatusExpired)
	return res.RowsAffected, res.Error
}

// StoreProductCoverage lists the store products and the cart totals with
// no active product, for the backoffice "Produk toko" page.
type StoreProductCoverageReport struct {
	Products      []models.StoreProduct `json:"products"`
	MissingTotals []int64               `json:"missing_totals"`
	MaxCartItems  int                   `json:"max_cart_items"`
	MaxCartTotal  int64                 `json:"max_cart_total_idr"`
	PriceStepIdr  int64                 `json:"price_step_idr"`
}

func (s *PaymentService) StoreProductCoverage() (*StoreProductCoverageReport, error) {
	var products []models.StoreProduct
	if err := s.db.Where("store = ?", models.StoreGoogle).Order("price_idr").Find(&products).Error; err != nil {
		return nil, err
	}
	active := map[int64]bool{}
	for _, p := range products {
		if p.Active {
			active[p.PriceIdr] = true
		}
	}
	report := &StoreProductCoverageReport{
		Products: products, MissingTotals: []int64{},
		MaxCartItems: CartMaxItems, MaxCartTotal: CartMaxTotalIdr, PriceStepIdr: CartPriceStepIdr,
	}
	for total := CartPriceStepIdr; total <= CartMaxTotalIdr; total += CartPriceStepIdr {
		if !active[total] {
			report.MissingTotals = append(report.MissingTotals, total)
		}
	}
	return report, nil
}
