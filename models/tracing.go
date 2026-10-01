package models

import (
	"github.com/google/uuid"
	"time"
)

type TracingItem struct {
	ID            uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Type          string    `gorm:"column:type;not null"                            json:"type"`
	Label         string    `gorm:"column:label;not null"                           json:"label"`
	GuidePathJSON string    `gorm:"column:guide_path_json;type:jsonb;not null"      json:"guide_path_json"`
	Difficulty    int       `gorm:"column:difficulty;not null;default:1"            json:"difficulty"`
	Hidden        bool      `gorm:"column:hidden;default:false"                     json:"hidden"`
	IsDeleted     bool      `gorm:"column:is_deleted;default:false"                 json:"-"`
	UpdatedAt     time.Time `gorm:"column:updated_at"                               json:"updated_at"`
	CreatedAt     time.Time `gorm:"column:created_at"                               json:"created_at"`
}

func (TracingItem) TableName() string { return "tracing_items" }

type TracingProgress struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"column:user_id;not null"                         json:"user_id"`
	ChildID   uuid.UUID `gorm:"column:child_id;not null"                        json:"child_id"`
	ItemID    uuid.UUID `gorm:"column:item_id;not null"                         json:"item_id"`
	Score     int       `gorm:"column:score;not null"                           json:"score"`
	Passed    bool      `gorm:"column:passed;not null;default:false"            json:"passed"`
	CreatedAt time.Time `gorm:"column:created_at"                               json:"created_at"`
}

func (TracingProgress) TableName() string { return "tracing_progress" }
