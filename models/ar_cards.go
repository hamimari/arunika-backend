package models

import (
	"gorm.io/gorm"
	"time"
)

type ArCards struct {
	ID        string     `gorm:"primaryKey;type:text" json:"id"`
	Type      string     `gorm:"type:text;not null" json:"type"`
	Title     string     `json:"title"`
	FileURL   string     `gorm:"type:text;not null" json:"file_url"`
	ShortCode string     `gorm:"uniqueIndex;type:text" json:"short_code"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func FindCardById(db *gorm.DB, id string) (*ArCards, error) {
	var arCard ArCards
	result := db.Where("id = ?", id).First(&arCard)
	if result.Error != nil {
		return nil, result.Error
	}
	return &arCard, nil
}
