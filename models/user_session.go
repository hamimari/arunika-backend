package models

import (
	"github.com/google/uuid"
	"time"
)

// UserSession records each authenticated login for DAU tracking.
type UserSession struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"column:user_id;not null"                         json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at"                               json:"created_at"`
}

func (UserSession) TableName() string { return "user_sessions" }
