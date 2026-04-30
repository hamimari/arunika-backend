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
	MidtransOrderID string     `gorm:"column:midtrans_order_id"                        json:"midtrans_order_id,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at"                               json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"                               json:"updated_at"`
}

func (UserSubscription) TableName() string { return "user_subscriptions" }
