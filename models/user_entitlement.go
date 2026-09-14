package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserEntitlement is a permanent, per-product grant of access for a user,
// created when a content-package order is paid. Subscription access is
// tracked separately on UserSubscription (blanket access), not here.
type UserEntitlement struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID  `gorm:"column:user_id;type:uuid;not null"              json:"user_id"`
	ProductID     uuid.UUID  `gorm:"column:product_id;type:uuid;not null"           json:"product_id"`
	StartsAt      time.Time  `gorm:"column:starts_at;not null"                      json:"starts_at"`
	ExpiresAt     *time.Time `gorm:"column:expires_at"                              json:"expires_at,omitempty"`
	SourceOrderID *uuid.UUID `gorm:"column:source_order_id;type:uuid"               json:"source_order_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (UserEntitlement) TableName() string { return "user_entitlements" }

// GrantEntitlement inserts a permanent entitlement for (userID, productID),
// idempotent on (user_id, product_id) — re-granting an existing entitlement
// is a no-op so webhook replays never duplicate/fail.
func GrantEntitlement(db *gorm.DB, userID, productID uuid.UUID, sourceOrderID *uuid.UUID) error {
	entitlement := UserEntitlement{
		UserID:        userID,
		ProductID:     productID,
		StartsAt:      time.Now(),
		SourceOrderID: sourceOrderID,
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entitlement).Error
}

// HasEntitlement reports whether the user has a non-expired entitlement for productID.
func HasEntitlement(db *gorm.DB, userID, productID uuid.UUID) (bool, error) {
	var count int64
	err := db.Model(&UserEntitlement{}).
		Where("user_id = ? AND product_id = ? AND (expires_at IS NULL OR expires_at > NOW())", userID, productID).
		Count(&count).Error
	return count > 0, err
}
