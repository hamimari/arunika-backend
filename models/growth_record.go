package models

import (
	"github.com/google/uuid"
	"time"
)

type GrowthRecord struct {
	ID         uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ChildID    uuid.UUID `gorm:"column:child_id;not null"                        json:"child_id"`
	RecordedAt time.Time `gorm:"column:recorded_at;not null"                     json:"recorded_at"`
	WeightKg   float64   `gorm:"column:weight_kg;not null"                       json:"weight_kg"`
	HeightCm   float64   `gorm:"column:height_cm;not null"                       json:"height_cm"`
	CreatedAt  time.Time `gorm:"column:created_at"                               json:"created_at"`
}

func (GrowthRecord) TableName() string { return "growth_records" }
