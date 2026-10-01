package models

import (
	"github.com/google/uuid"
	"time"
)

// Token stores a SHA-256 hash of the token emailed to the user (see
// utils.HashResetToken), never the raw value — so reading this table alone
// never yields a directly usable reset link.
type PasswordResetToken struct {
	BaseModel
	UserID    uuid.UUID `gorm:"index"`
	Token     string    `gorm:"uniqueIndex"`
	ExpiresAt time.Time
}
