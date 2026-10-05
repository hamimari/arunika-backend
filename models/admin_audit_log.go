package models

import (
	"time"

	"github.com/google/uuid"
)

// AdminAuditLog records who changed what in the backoffice, with the value
// before and after the change.
type AdminAuditLog struct {
	ID         int64     `gorm:"primaryKey"                json:"id"`
	AdminID    uuid.UUID `gorm:"type:uuid;not null"        json:"admin_id"`
	Action     string    `gorm:"not null"                  json:"action"`
	EntityType string    `gorm:"not null"                  json:"entity_type"`
	EntityID   string    `gorm:"not null"                  json:"entity_id"`
	Before     JSONB     `gorm:"type:jsonb"                json:"before"`
	After      JSONB     `gorm:"type:jsonb"                json:"after"`
	CreatedAt  time.Time `gorm:"not null;autoCreateTime"   json:"created_at"`
}

func (AdminAuditLog) TableName() string { return "admin_audit_log" }
