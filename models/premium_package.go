package models

import (
	"time"

	"gorm.io/gorm"
)

type PremiumPackage struct {
	ID          string  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name        string  `gorm:"type:varchar(100);not null"                     json:"name"`
	Subtitle    string  `gorm:"type:varchar(255);not null"                     json:"subtitle"`
	Description *string `gorm:"type:text"                                      json:"description"`
	ImageURL    *string `gorm:"column:image_url;type:text"                     json:"image_url"`
	PriceIdr    int     `gorm:"not null" json:"price_idr"`
	Type        string  `gorm:"type:varchar(20);not null"                      json:"type"`
	BadgeLabel  *string `gorm:"type:varchar(50)"                               json:"badge_label"`
	IsBestValue bool    `gorm:"not null;default:false"                         json:"is_best_value"`
	IsActive    bool    `gorm:"not null;default:true"                          json:"is_active"`
	SortOrder   int     `gorm:"not null;default:0"                             json:"sort_order"`
	// DurationDays is required when Type == "subscription" (enforced by a DB
	// CHECK constraint) and NULL for Type == "content".
	DurationDays *int      `gorm:"column:duration_days" json:"duration_days"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (PremiumPackage) TableName() string {
	return "premium_packages"
}

// FindActivePremiumPackages returns all active packages ordered by sort_order.
// When packType is empty, no type filter is applied and packages of every type
// are returned; otherwise results are restricted to that type.
func FindActivePremiumPackages(db *gorm.DB, packType string) ([]PremiumPackage, error) {
	var packs []PremiumPackage
	query := db.Where("is_active = true")
	if packType != "" {
		query = query.Where("type = ?", packType)
	}
	result := query.Order("sort_order asc").Find(&packs)
	return packs, result.Error
}

// FindAllPremiumPackages returns all packages (including inactive), ordered by sort_order.
func FindAllPremiumPackages(db *gorm.DB) ([]PremiumPackage, error) {
	var packs []PremiumPackage
	result := db.Order("sort_order asc").Find(&packs)
	return packs, result.Error
}

// FindPremiumPackageByID returns a single package by ID.
func FindPremiumPackageByID(db *gorm.DB, id string) (*PremiumPackage, error) {
	var pack PremiumPackage
	result := db.First(&pack, "id = ?", id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &pack, nil
}

func FindPremiumPackageByName(db *gorm.DB, id string) (*PremiumPackage, error) {
	var pack PremiumPackage
	result := db.First(&pack, "name = ?", id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &pack, nil
}
