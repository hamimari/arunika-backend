package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type ArCards struct {
	ID        string `gorm:"primaryKey;type:text"       json:"id"`
	Type      string `gorm:"type:text;not null"         json:"type"`
	Title     string `json:"title"`
	FileURL   string `gorm:"type:text;not null"         json:"file_url"`
	SoundUrl  string `gorm:"column:sound_url;type:text" json:"sound_url"`
	ShortCode string `gorm:"uniqueIndex;type:text"      json:"short_code"`
	Hidden    bool   `gorm:"column:hidden;default:false" json:"hidden"`
	// Legacy free-text fields (kept for backward compat, prefer CategoryID/SubCategoryID)
	Category     string `gorm:"type:varchar(50)"           json:"category"`
	SubCategory  string `gorm:"column:sub_category;type:varchar(50)" json:"sub_category"`
	ImageURL     string `gorm:"column:image_url;type:text" json:"image_url"`
	Emoji        string `gorm:"type:varchar(20)"           json:"emoji"`
	BgColor      string `gorm:"column:bg_color;type:varchar(20);default:'#FFF3E0'" json:"bg_color"`
	IsUnlocked   bool   `gorm:"column:is_unlocked;default:false" json:"is_unlocked"`
	Description  string `gorm:"type:text"                  json:"description"`
	PrintableImg string `gorm:"column:printable_img;type:text;default:''" json:"printable_img"`
	// Structured category FKs (from V12 migration)
	CategoryID     *uuid.UUID      `gorm:"column:category_id;type:uuid"     json:"category_id,omitempty"`
	SubCategoryID  *uuid.UUID      `gorm:"column:sub_category_id;type:uuid" json:"sub_category_id,omitempty"`
	CategoryRef    *ArCardCategory `gorm:"foreignKey:CategoryID"            json:"category_ref,omitempty"`
	SubCategoryRef *ArCardCategory `gorm:"foreignKey:SubCategoryID"         json:"sub_category_ref,omitempty"`
	IsDeleted      bool            `gorm:"column:is_deleted;default:false"  json:"-"`
	UpdatedAt      time.Time       `json:"updated_at"`
	CreatedAt      time.Time       `json:"created_at"`
	ExpiresAt      *time.Time      `json:"expires_at,omitempty"`
}

func FindCardById(db *gorm.DB, id string) (*ArCards, error) {
	var arCard ArCards
	result := db.Preload("CategoryRef").Preload("SubCategoryRef").Where("id = ?", id).First(&arCard)
	if result.Error != nil {
		return nil, result.Error
	}
	return &arCard, nil
}

func FindAllCards(db *gorm.DB, categoryID, subCategoryID string) ([]ArCards, error) {
	query := db.Model(&ArCards{}).Preload("CategoryRef").Preload("SubCategoryRef")
	if categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}
	if subCategoryID != "" {
		query = query.Where("sub_category_id = ?", subCategoryID)
	}
	var cards []ArCards
	if err := query.Find(&cards).Error; err != nil {
		return nil, err
	}
	return cards, nil
}
