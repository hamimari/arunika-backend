package models

import "time"

const (
	FeatureFlagPrintableCards = "printable_cards"
	FeatureFlagQRScan         = "qr_scan"
)

// FeatureFlag is a remote on/off switch for an app feature (see V44).
type FeatureFlag struct {
	Key         string    `gorm:"column:key;primaryKey"                json:"key"`
	Name        string    `gorm:"column:name;not null"                 json:"name"`
	Description string    `gorm:"column:description;not null"          json:"description"`
	IsEnabled   bool      `gorm:"column:is_enabled;not null;default:true" json:"is_enabled"`
	UpdatedAt   time.Time `gorm:"column:updated_at"                    json:"updated_at"`
}

func (FeatureFlag) TableName() string { return "app_feature_flags" }
