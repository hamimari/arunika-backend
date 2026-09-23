package services

import (
	"arunika_backend/models"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrOrderForbidden = errors.New("order does not belong to this user")

type OrderService struct {
	db             *gorm.DB
	productService *ProductService
}

func NewOrderService(db *gorm.DB, productService *ProductService) *OrderService {
	return &OrderService{db: db, productService: productService}
}

// AdminOrderView is an Order enriched with the buyer's contact info and the
// human-readable name of whatever was bought — the admin UI has no use for
// bare user_id/product_id/package_id.
type AdminOrderView struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	UserName    string     `json:"user_name"`
	UserEmail   string     `json:"user_email"`
	UserPhone   string     `json:"user_phone"`
	ProductID   *uuid.UUID `json:"product_id,omitempty"`
	ProductName *string    `json:"product_name,omitempty"`
	PackageID   *uuid.UUID `json:"package_id,omitempty"`
	PackageName *string    `json:"package_name,omitempty"`
	AmountIdr   int64      `json:"amount_idr"`
	Status      string     `json:"status"`
	// Provider is which payment rail this order was created against
	// ("midtrans" | "google_play") — drives which single sync action the
	// backoffice offers for it.
	Provider string `json:"provider"`
	// HasPurchaseToken reports whether a Google Play purchase token is on
	// file for this order (never the token itself) — lets the backoffice
	// sync a Play order in one click instead of asking an admin to paste
	// one in manually.
	HasPurchaseToken bool      `json:"has_purchase_token"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// List returns a paginated, optionally status-filtered and searched list of
// orders (admin use). search matches against the buyer's name, email, phone
// number, user id, or the order id itself.
func (s *OrderService) List(status, search string, page, perPage int) ([]AdminOrderView, int64, error) {
	var items []models.Order
	var total int64

	q := s.db.Model(&models.Order{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if search != "" {
		like := "%" + search + "%"
		q = q.Joins("JOIN parents p ON p.id = orders.user_id").
			Where("p.name ILIKE ? OR p.email_address ILIKE ? OR p.phone_number ILIKE ? OR orders.user_id::text ILIKE ? OR orders.id::text ILIKE ?",
				like, like, like, like, like)
	}

	q.Count(&total)
	if search != "" {
		q = q.Select("orders.*")
	}
	err := q.Order("orders.created_at DESC").
		Limit(perPage).Offset((page - 1) * perPage).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	views, err := s.enrichOrders(items)
	return views, total, err
}

// enrichOrders batch-loads buyer and package info (avoiding N+1 queries) and
// resolves each product's display name via ProductService.ResolveDisplayName.
func (s *OrderService) enrichOrders(orders []models.Order) ([]AdminOrderView, error) {
	views := make([]AdminOrderView, len(orders))
	if len(orders) == 0 {
		return views, nil
	}

	userIDSet := make(map[uuid.UUID]bool)
	packageIDSet := make(map[string]bool)
	for _, o := range orders {
		userIDSet[o.UserID] = true
		if o.PackageID != nil {
			packageIDSet[o.PackageID.String()] = true
		}
	}
	userIDs := make([]uuid.UUID, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}
	packageIDs := make([]string, 0, len(packageIDSet))
	for id := range packageIDSet {
		packageIDs = append(packageIDs, id)
	}

	var parents []models.Parent
	if err := s.db.Where("id IN ?", userIDs).Find(&parents).Error; err != nil {
		return nil, err
	}
	parentByID := make(map[uuid.UUID]models.Parent, len(parents))
	for _, p := range parents {
		parentByID[p.ID] = p
	}

	packageNameByID := make(map[string]string)
	if len(packageIDs) > 0 {
		var packages []models.PremiumPackage
		if err := s.db.Where("id IN ?", packageIDs).Find(&packages).Error; err != nil {
			return nil, err
		}
		for _, pkg := range packages {
			packageNameByID[pkg.ID] = pkg.Name
		}
	}

	for i, o := range orders {
		view := AdminOrderView{
			ID:               o.ID,
			UserID:           o.UserID,
			ProductID:        o.ProductID,
			PackageID:        o.PackageID,
			AmountIdr:        o.AmountIdr,
			Status:           o.Status,
			Provider:         o.Provider,
			HasPurchaseToken: o.PurchaseToken != nil && *o.PurchaseToken != "",
			CreatedAt:        o.CreatedAt,
			UpdatedAt:        o.UpdatedAt,
		}
		if parent, ok := parentByID[o.UserID]; ok {
			view.UserName = parent.Name
			view.UserEmail = parent.EmailAddress
			view.UserPhone = parent.PhoneNumber
		}
		if o.ProductID != nil {
			if name, err := s.productService.ResolveDisplayName(*o.ProductID); err == nil {
				view.ProductName = &name
			}
		}
		if o.PackageID != nil {
			if name, ok := packageNameByID[o.PackageID.String()]; ok {
				view.PackageName = &name
			}
		}
		views[i] = view
	}
	return views, nil
}

// PurchasedPackageIDs returns the set of premium_package IDs userID holds a
// PAID order for — used to hide already-bought content packages from the
// catalog (a content package is a one-time, permanent purchase, so there's
// no reason to offer it again).
func (s *OrderService) PurchasedPackageIDs(userID uuid.UUID) (map[string]bool, error) {
	var packageIDs []string
	err := s.db.Model(&models.Order{}).
		Where("user_id = ? AND status = ? AND package_id IS NOT NULL", userID, models.OrderStatusPaid).
		Pluck("package_id", &packageIDs).Error
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(packageIDs))
	for _, id := range packageIDs {
		set[id] = true
	}
	return set, nil
}

// GetByID returns a single order by ID (admin use — no ownership check).
func (s *OrderService) GetByID(id uuid.UUID) (*models.Order, error) {
	return models.FindOrderByID(s.db, id)
}

// EnrichOne is enrichOrders for a single order, used after actions (e.g. a
// Midtrans sync) that need to return the updated row in admin view shape.
func (s *OrderService) EnrichOne(order models.Order) (AdminOrderView, error) {
	views, err := s.enrichOrders([]models.Order{order})
	if err != nil || len(views) == 0 {
		return AdminOrderView{}, err
	}
	return views[0], nil
}

// GetOwnedOrder returns the order if it exists and belongs to userID.
// Returns ErrOrderForbidden (not gorm.ErrRecordNotFound) for orders that
// exist but belong to someone else, so the handler can 404 either way
// without leaking which case it was.
func (s *OrderService) GetOwnedOrder(orderID, userID uuid.UUID) (*models.Order, error) {
	order, err := models.FindOrderByID(s.db, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, ErrOrderForbidden
	}
	return order, nil
}

// Item types reported on UserOrderView.ItemType.
const (
	OrderItemArCard  = "AR_CARD"
	OrderItemDongeng = "DONGENG"
	OrderItemPackage = "PACKAGE"
)

// UserOrderView is one row of a user's own payment history: an order plus
// the name/type of what was bought and how it was paid.
type UserOrderView struct {
	ID            uuid.UUID `json:"id"`
	ItemType      string    `json:"item_type"` // AR_CARD | DONGENG | PACKAGE | "" (content since deleted)
	ItemName      string    `json:"item_name"`
	PackageType   string    `json:"package_type,omitempty"` // content | subscription, PACKAGE only
	AmountIdr     int64     `json:"amount_idr"`
	Status        string    `json:"status"`
	PaymentType   string    `json:"payment_type"`   // raw Midtrans payment_type, "" until the user picks a method
	PaymentMethod string    `json:"payment_method"` // human-readable label for PaymentType
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ListOrdersForUser returns one page of userID's orders (every status),
// newest first.
func (s *OrderService) ListOrdersForUser(userID uuid.UUID, page, perPage int) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64
	q := s.db.Model(&models.Order{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").
		Limit(perPage).Offset((page - 1) * perPage).
		Find(&orders).Error
	return orders, total, err
}

// EnrichForUser resolves item names/types and payment methods for a page of
// orders with a fixed number of batch queries (no per-order lookups).
func (s *OrderService) EnrichForUser(orders []models.Order) ([]UserOrderView, error) {
	views := make([]UserOrderView, len(orders))
	if len(orders) == 0 {
		return views, nil
	}

	orderIDs := make([]uuid.UUID, 0, len(orders))
	productIDs := make([]uuid.UUID, 0)
	packageIDs := make([]uuid.UUID, 0)
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
		if o.ProductID != nil {
			productIDs = append(productIDs, *o.ProductID)
		}
		if o.PackageID != nil {
			packageIDs = append(packageIDs, *o.PackageID)
		}
	}

	type productRow struct {
		ProductID string
		Title     string
	}
	arCardByProduct := map[string]string{}
	dongengByProduct := map[string]string{}
	if len(productIDs) > 0 {
		var rows []productRow
		if err := s.db.Table("product_ar_cards pac").
			Select("pac.product_id::text AS product_id, a.title AS title").
			Joins("JOIN ar_cards a ON a.id = pac.ar_card_id").
			Where("pac.product_id IN ?", productIDs).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			arCardByProduct[r.ProductID] = r.Title
		}
		rows = nil
		if err := s.db.Table("product_dongengs pd").
			Select("pd.product_id::text AS product_id, d.title AS title").
			Joins("JOIN dongengs d ON d.id = pd.dongeng_id").
			Where("pd.product_id IN ?", productIDs).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			dongengByProduct[r.ProductID] = r.Title
		}
	}

	packageByID := map[string]models.PremiumPackage{}
	if len(packageIDs) > 0 {
		var packages []models.PremiumPackage
		if err := s.db.Where("id IN ?", packageIDs).Find(&packages).Error; err != nil {
			return nil, err
		}
		for _, p := range packages {
			packageByID[p.ID] = p
		}
	}

	// Every payment row that recorded a method, newest first. The initial
	// row written at checkout has an empty payment_type, so it's skipped.
	var payments []models.Payment
	if err := s.db.Select("order_id", "payment_type", "raw_payload", "created_at").
		Where("order_id IN ? AND payment_type IS NOT NULL AND payment_type <> ''", orderIDs).
		Order("created_at DESC").
		Find(&payments).Error; err != nil {
		return nil, err
	}
	type method struct {
		paymentType, label string
		specific           bool
	}
	methodByOrder := map[uuid.UUID]method{}
	for _, p := range payments {
		label, specific := PaymentMethodLabel(p.PaymentType, p.RawPayload)
		// Keep the newest row, unless an older one names the bank/store
		// and the newest doesn't (a status-sync row carries less detail
		// than the original webhook payload).
		if cur, ok := methodByOrder[p.OrderID]; ok && (cur.specific || !specific) {
			continue
		}
		methodByOrder[p.OrderID] = method{paymentType: p.PaymentType, label: label, specific: specific}
	}

	for i, o := range orders {
		view := UserOrderView{
			ID:        o.ID,
			AmountIdr: o.AmountIdr,
			Status:    o.Status,
			CreatedAt: o.CreatedAt,
			UpdatedAt: o.UpdatedAt,
		}
		switch {
		case o.PackageID != nil:
			view.ItemType = OrderItemPackage
			if pkg, ok := packageByID[o.PackageID.String()]; ok {
				view.ItemName = pkg.Name
				view.PackageType = pkg.Type
			}
		case o.ProductID != nil:
			if title, ok := arCardByProduct[o.ProductID.String()]; ok {
				view.ItemType = OrderItemArCard
				view.ItemName = title
			} else if title, ok := dongengByProduct[o.ProductID.String()]; ok {
				view.ItemType = OrderItemDongeng
				view.ItemName = title
			}
		}
		if m, ok := methodByOrder[o.ID]; ok {
			view.PaymentType = m.paymentType
			view.PaymentMethod = m.label
		}
		views[i] = view
	}
	return views, nil
}
