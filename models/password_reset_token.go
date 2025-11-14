package models

import (
	"github.com/google/uuid"
	"time"
)

type PasswordResetToken struct {
	BaseModel
	UserID    uuid.UUID `gorm:"index"`
	Token     string    `gorm:"uniqueIndex"`
	ExpiresAt time.Time
}
