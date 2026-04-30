package models

import (
	"github.com/google/uuid"
	"time"
)

type Badge struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Feature   string    `gorm:"column:feature;not null"                         json:"feature"`
	Level     string    `gorm:"column:level;not null"                           json:"level"`
	Threshold int       `gorm:"column:threshold;not null"                       json:"threshold"`
	Hidden    bool      `gorm:"column:hidden;default:false"                     json:"hidden"`
	IsDeleted bool      `gorm:"column:is_deleted;default:false"                 json:"-"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"                json:"updated_at"`
}

func (Badge) TableName() string { return "badges" }

type UserBadge struct {
	ID       uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID   uuid.UUID `gorm:"column:user_id;not null"                         json:"user_id"`
	BadgeID  uuid.UUID `gorm:"column:badge_id;not null"                        json:"badge_id"`
	EarnedAt time.Time `gorm:"column:earned_at"                                json:"earned_at"`
	Badge    Badge     `gorm:"foreignKey:BadgeID"                              json:"badge,omitempty"`
}

func (UserBadge) TableName() string { return "user_badges" }
