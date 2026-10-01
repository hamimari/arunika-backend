package models

import (
	"github.com/google/uuid"
	"time"
)

type Notification struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"column:user_id;not null"                         json:"user_id"`
	Title     string    `gorm:"column:title;not null"                           json:"title"`
	Body      string    `gorm:"column:body;not null"                            json:"body"`
	Type      string    `gorm:"column:type;not null"                            json:"type"`
	IsRead    bool      `gorm:"column:is_read;not null;default:false"           json:"is_read"`
	CreatedAt time.Time `gorm:"column:created_at"                               json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

type FCMToken struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"column:user_id;not null"                         json:"user_id"`
	Token     string    `gorm:"column:token;not null;uniqueIndex"               json:"token"`
	CreatedAt time.Time `gorm:"column:created_at"                               json:"created_at"`
}

func (FCMToken) TableName() string { return "fcm_tokens" }
