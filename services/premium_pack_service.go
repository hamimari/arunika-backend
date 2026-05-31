package services

import (
	"arunika_backend/models"
	"gorm.io/gorm"
)

type PremiumPackService struct {
	db *gorm.DB
}

func NewPremiumPackService(db *gorm.DB) *PremiumPackService {
	return &PremiumPackService{db: db}
}

// GetActivePacks returns active packages, optionally filtered by type.
func (s *PremiumPackService) GetActivePacks(packType string) ([]models.PremiumPackage, error) {
	return models.FindActivePremiumPackages(s.db, packType)
}

// GetAllPacks returns all packages including inactive (admin use).
func (s *PremiumPackService) GetAllPacks() ([]models.PremiumPackage, error) {
	return models.FindAllPremiumPackages(s.db)
}

type CreatePremiumPackInput struct {
	Name        string  `json:"name"          binding:"required"`
	Subtitle    string  `json:"subtitle"      binding:"required"`
	PriceIDR    int     `json:"price_idr"     binding:"required,min=1"`
	Type        string  `json:"type"          binding:"required,oneof=content subscription"`
	BadgeLabel  *string `json:"badge_label"`
	IsBestValue bool    `json:"is_best_value"`
	SortOrder   int     `json:"sort_order"`
}

// CreatePack inserts a new premium package.
func (s *PremiumPackService) CreatePack(input CreatePremiumPackInput) (*models.PremiumPackage, error) {
	pack := models.PremiumPackage{
		Name:        input.Name,
		Subtitle:    input.Subtitle,
		PriceIDR:    input.PriceIDR,
		Type:        input.Type,
		BadgeLabel:  input.BadgeLabel,
		IsBestValue: input.IsBestValue,
		SortOrder:   input.SortOrder,
		IsActive:    true,
	}
	result := s.db.Create(&pack)
	return &pack, result.Error
}

type UpdatePremiumPackInput struct {
	Name        string  `json:"name"          binding:"required"`
	Subtitle    string  `json:"subtitle"      binding:"required"`
	PriceIDR    int     `json:"price_idr"     binding:"required,min=1"`
	Type        string  `json:"type"          binding:"required,oneof=content subscription"`
	BadgeLabel  *string `json:"badge_label"`
	IsBestValue bool    `json:"is_best_value"`
	SortOrder   int     `json:"sort_order"`
}

// UpdatePack updates an existing premium package. Returns nil if not found.
func (s *PremiumPackService) UpdatePack(id string, input UpdatePremiumPackInput) (*models.PremiumPackage, error) {
	pack, err := models.FindPremiumPackageByID(s.db, id)
	if err != nil {
		return nil, err
	}
	pack.Name = input.Name
	pack.Subtitle = input.Subtitle
	pack.PriceIDR = input.PriceIDR
	pack.Type = input.Type
	pack.BadgeLabel = input.BadgeLabel
	pack.IsBestValue = input.IsBestValue
	pack.SortOrder = input.SortOrder
	result := s.db.Save(pack)
	return pack, result.Error
}

// DeletePack removes a premium package by ID. Returns gorm.ErrRecordNotFound if missing.
func (s *PremiumPackService) DeletePack(id string) error {
	result := s.db.Delete(&models.PremiumPackage{}, "id = ?", id)
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

type ToggleVisibilityInput struct {
	IsActive bool `json:"is_active"`
}

// ToggleVisibility updates the is_active field for a package.
func (s *PremiumPackService) ToggleVisibility(id string, isActive bool) (*models.PremiumPackage, error) {
	if err := s.db.Model(&models.PremiumPackage{}).Where("id = ?", id).Update("is_active", isActive).Error; err != nil {
		return nil, err
	}
	return models.FindPremiumPackageByID(s.db, id)
}
