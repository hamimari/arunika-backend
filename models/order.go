package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	OrderStatusPending = "PENDING"
	OrderStatusPaid    = "PAID"
	OrderStatusFailed  = "FAILED"
	OrderStatusExpired = "EXPIRED"
	// OrderStatusRefunded marks a previously-PAID order whose purchase
	// Google Play has since voided — a real refund/chargeback, or Google's
	// own automatic refund of a Play Billing purchase left unacknowledged
	// for 3 days. Distinct from EXPIRED, which means the order was never
	// paid at all (its Midtrans Snap link lapsed unused).
	OrderStatusRefunded = "REFUNDED"
)

const (
	OrderProviderMidtrans   = "midtrans"
	OrderProviderGooglePlay = "google_play"
)

// Order is created (status PENDING) before a Midtrans Snap transaction and
// drives payment settlement + entitlement granting. Exactly one of
// ProductID/PackageID is set, matching the DB CHECK constraint.
type Order struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID  `gorm:"column:user_id;type:uuid;not null"              json:"user_id"`
	ProductID *uuid.UUID `gorm:"column:product_id;type:uuid"                    json:"product_id,omitempty"`
	PackageID *uuid.UUID `gorm:"column:package_id;type:uuid"                    json:"package_id,omitempty"`
	AmountIdr int64      `gorm:"column:amount_idr;not null"                     json:"amount_idr"`
	Status    string     `gorm:"column:status;not null;default:PENDING"         json:"status"`
	// Provider is which payment rail this order was created against —
	// drives which sync action the backoffice offers for it.
	Provider string `gorm:"column:provider;not null;default:midtrans" json:"provider"`
	// PurchaseToken is the Google Play purchase token, recorded as soon as
	// the app reports it (regardless of whether verification against
	// Google succeeds) so a later admin-triggered re-sync never needs it
	// typed in by hand. Nil for Midtrans orders. Never serialized directly
	// — see AdminOrderView.HasPurchaseToken.
	PurchaseToken *string   `gorm:"column:purchase_token" json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Order) TableName() string { return "orders" }

func FindOrderByID(db *gorm.DB, id uuid.UUID) (*Order, error) {
	var order Order
	if err := db.Where("id = ?", id).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}
