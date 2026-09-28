package models

import (
	"github.com/google/uuid"
	"time"
)

type UserSubscription struct {
	ID              uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID          uuid.UUID  `gorm:"column:user_id;not null;uniqueIndex"             json:"user_id"`
	Status          string     `gorm:"column:status;not null;default:free"             json:"status"`
	ExpiresAt       *time.Time `gorm:"column:expires_at"                               json:"expires_at,omitempty"`
	ProviderOrderID string     `gorm:"column:provider_order_id"                        json:"provider_order_id,omitempty"`
	// PackageID is nullable — the admin manual-grant path sets status/expiry
	// without a purchase and must keep working without one.
	PackageID *uuid.UUID `gorm:"column:package_id;type:uuid"       json:"package_id,omitempty"`
	StartDate *time.Time `gorm:"column:start_date"                 json:"start_date,omitempty"`
	AutoRenew bool       `gorm:"column:auto_renew;not null;default:false" json:"auto_renew"`
	// Provider is the payment rail managing this subscription (V57):
	// OrderProviderMidtrans or OrderProviderGooglePlay.
	Provider  string    `gorm:"column:provider;not null;default:midtrans" json:"provider"`
	CreatedAt time.Time `gorm:"column:created_at"                 json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"                 json:"updated_at"`
}

func (UserSubscription) TableName() string { return "user_subscriptions" }
