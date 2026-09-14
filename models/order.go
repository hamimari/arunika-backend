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
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (Order) TableName() string { return "orders" }

func FindOrderByID(db *gorm.DB, id uuid.UUID) (*Order, error) {
	var order Order
	if err := db.Where("id = ?", id).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}
