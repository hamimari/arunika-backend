package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	AssetKindImage = "image"
	AssetKindAudio = "audio"
)

// ContentAsset is an uploaded image or audio file, stored content-addressed
// by its sha256 (see services.AssetService).
type ContentAsset struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Kind         string    `gorm:"not null"                                        json:"kind"`
	StorageKey   string    `gorm:"not null"                                        json:"-"`
	Sha256       string    `gorm:"column:sha256;not null;uniqueIndex"              json:"sha256"`
	Bytes        int       `gorm:"not null"                                        json:"bytes"`
	Mime         string    `gorm:"not null"                                        json:"mime"`
	Width        *int      `json:"width"`
	Height       *int      `json:"height"`
	DurationMs   *int      `json:"duration_ms"`
	OriginalName string    `gorm:"not null;default:''"                             json:"original_name"`
	CreatedBy    uuid.UUID `gorm:"type:uuid;not null"                              json:"-"`
	CreatedAt    time.Time `gorm:"autoCreateTime"                                  json:"created_at"`
}

func (ContentAsset) TableName() string { return "content_assets" }
