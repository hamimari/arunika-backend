package models

import (
	"gorm.io/gorm"
	"time"
)

type PremiumPackage struct {
	ID          string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null"                     json:"name"`
	Subtitle    string    `gorm:"type:varchar(255);not null"                     json:"subtitle"`
	PriceIDR    int       `gorm:"not null"                                       json:"price_idr"`
	Type        string    `gorm:"type:varchar(20);not null"                      json:"type"`
	BadgeLabel  *string   `gorm:"type:varchar(50)"                               json:"badge_label"`
	IsBestValue bool      `gorm:"not null;default:false"                         json:"is_best_value"`
	IsActive    bool      `gorm:"not null;default:true"                          json:"is_active"`
	SortOrder   int       `gorm:"not null;default:0"                             json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (PremiumPackage) TableName() string {
	return "premium_packages"
}

// FindActivePremiumPackages returns all active packages ordered by sort_order.
// The packType parameter is intentionally ignored — the public endpoint always
// returns every active pack (both subscription and content) so the Flutter app
// can determine banner visibility based on the full active set.
func FindActivePremiumPackages(db *gorm.DB, packType string) ([]PremiumPackage, error) {
	var packs []PremiumPackage
	result := db.Where("is_active = true").Order("sort_order asc").Find(&packs)
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
