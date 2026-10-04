package models

import (
	"time"

	"github.com/google/uuid"
)

// GrowthMeasurement is one height/weight measurement of a child. HfaZ/WfaZ are
// internal (API and analytics only); the app shows categories, never z-scores.
type GrowthMeasurement struct {
	ID          uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	ChildID     uuid.UUID  `gorm:"column:child_id;not null"                        json:"child_id"`
	MeasuredOn  time.Time  `gorm:"column:measured_on;type:date;not null"           json:"measured_on"`
	HeightCm    *float64   `gorm:"column:height_cm"                                json:"height_cm"`
	WeightKg    *float64   `gorm:"column:weight_kg"                                json:"weight_kg"`
	Position    string     `gorm:"column:position;not null"                        json:"position"`
	AgeDays     int        `gorm:"column:age_days;not null"                        json:"age_days"`
	HfaZ        *float64   `gorm:"column:hfa_z"                                    json:"hfa_z"`
	WfaZ        *float64   `gorm:"column:wfa_z"                                    json:"wfa_z"`
	HfaCategory *string    `gorm:"column:hfa_category"                             json:"hfa_category"`
	WfaCategory *string    `gorm:"column:wfa_category"                             json:"wfa_category"`
	Flagged     bool       `gorm:"column:flagged;not null"                         json:"flagged"`
	WHOVersion  string     `gorm:"column:who_version;not null;default:who2006"     json:"who_version"`
	ClientID    *uuid.UUID `gorm:"column:client_id"                                json:"client_id"`
	CreatedAt   time.Time  `gorm:"column:created_at"                               json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"                               json:"updated_at"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"                               json:"-"`
}

func (GrowthMeasurement) TableName() string { return "growth_measurements" }
