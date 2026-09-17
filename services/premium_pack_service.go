package services

import (
	"arunika_backend/models"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PremiumPackService struct {
	db           *gorm.DB
	orderService *OrderService
}

func NewPremiumPackService(db *gorm.DB, orderService *OrderService) *PremiumPackService {
	return &PremiumPackService{db: db, orderService: orderService}
}

// GetActivePacks returns active packages, optionally filtered by type. When
// userID is non-nil, content packages the user already holds a PAID order
// for are excluded — a content package is a one-time, permanent purchase,
// so there's no reason to re-offer it. Subscription packages are always
// included so the user can renew/resubscribe.
func (s *PremiumPackService) GetActivePacks(packType string, userID *uuid.UUID) ([]models.PremiumPackage, error) {
	packs, err := models.FindActivePremiumPackages(s.db, packType)
	if err != nil {
		return nil, err
	}
	if userID == nil {
		return packs, nil
	}

	purchased, err := s.orderService.PurchasedPackageIDs(*userID)
	if err != nil {
		return nil, err
	}

	filtered := packs[:0]
	for _, p := range packs {
		if p.Type == "content" && purchased[p.ID] {
			continue
		}
		filtered = append(filtered, p)
	}
	return filtered, nil
}

// GetAllPacks returns all packages including inactive (admin use).
func (s *PremiumPackService) GetAllPacks() ([]models.PremiumPackage, error) {
	return models.FindAllPremiumPackages(s.db)
}

type CreatePremiumPackInput struct {
	Name         string  `json:"name"          binding:"required"`
	Subtitle     string  `json:"subtitle"      binding:"required"`
	Description  *string `json:"description"`
	ImageURL     *string `json:"image_url"`
	PriceIdr     int     `json:"price_idr" binding:"required,min=1"`
	Type         string  `json:"type"          binding:"required,oneof=content subscription"`
	BadgeLabel   *string `json:"badge_label"`
	IsBestValue  bool    `json:"is_best_value"`
	SortOrder    int     `json:"sort_order"`
	DurationDays *int    `json:"duration_days"`
}

// validateDurationDays enforces that duration_days is set (and positive) for
// subscription packs, and unset for content packs — mirrors the DB CHECK
// constraint so callers get a clean 400 instead of a raw SQL error.
func validateDurationDays(packType string, durationDays *int) error {
	if packType == "subscription" {
		if durationDays == nil || *durationDays <= 0 {
			return errors.New("duration_days is required and must be positive for subscription packages")
		}
	}
	return nil
}

// CreatePack inserts a new premium package.
func (s *PremiumPackService) CreatePack(input CreatePremiumPackInput) (*models.PremiumPackage, error) {
	if err := validateDurationDays(input.Type, input.DurationDays); err != nil {
		return nil, err
	}
	pack := models.PremiumPackage{
		Name:         input.Name,
		Subtitle:     input.Subtitle,
		Description:  input.Description,
		ImageURL:     input.ImageURL,
		PriceIdr:     input.PriceIdr,
		Type:         input.Type,
		BadgeLabel:   input.BadgeLabel,
		IsBestValue:  input.IsBestValue,
		SortOrder:    input.SortOrder,
		DurationDays: input.DurationDays,
		IsActive:     true,
	}
	result := s.db.Create(&pack)
	return &pack, result.Error
}

type UpdatePremiumPackInput struct {
	Name         string  `json:"name"          binding:"required"`
	Subtitle     string  `json:"subtitle"      binding:"required"`
	Description  *string `json:"description"`
	ImageURL     *string `json:"image_url"`
	PriceIDR     int     `json:"price_idr"     binding:"required,min=1"`
	Type         string  `json:"type"          binding:"required,oneof=content subscription"`
	BadgeLabel   *string `json:"badge_label"`
	IsBestValue  bool    `json:"is_best_value"`
	SortOrder    int     `json:"sort_order"`
	DurationDays *int    `json:"duration_days"`
}

// UpdatePack updates an existing premium package. Returns nil if not found.
func (s *PremiumPackService) UpdatePack(id string, input UpdatePremiumPackInput) (*models.PremiumPackage, error) {
	if err := validateDurationDays(input.Type, input.DurationDays); err != nil {
		return nil, err
	}
	pack, err := models.FindPremiumPackageByID(s.db, id)
	if err != nil {
		return nil, err
	}
	pack.Name = input.Name
	pack.Subtitle = input.Subtitle
	pack.Description = input.Description
	pack.ImageURL = input.ImageURL
	pack.PriceIdr = input.PriceIDR
	pack.Type = input.Type
	pack.BadgeLabel = input.BadgeLabel
	pack.IsBestValue = input.IsBestValue
	pack.SortOrder = input.SortOrder
	pack.DurationDays = input.DurationDays
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

func (s *PremiumPackService) GetByName(name string) (*models.PremiumPackage, error) {
	return models.FindPremiumPackageByName(s.db, name)
}

// ListItems returns every product bundled in a package.
func (s *PremiumPackService) ListItems(packageID string) ([]models.PremiumPackageItem, error) {
	id, err := uuid.Parse(packageID)
	if err != nil {
		return nil, errors.New("invalid package id")
	}
	return models.FindPackageItems(s.db, id)
}

// AddItem bundles a product into a package. Idempotent.
func (s *PremiumPackService) AddItem(packageID, productID string) error {
	pid, err := uuid.Parse(packageID)
	if err != nil {
		return errors.New("invalid package id")
	}
	prodID, err := uuid.Parse(productID)
	if err != nil {
		return errors.New("invalid product id")
	}
	return models.AddPackageItem(s.db, pid, prodID)
}

// RemoveItem un-bundles a product from a package.
func (s *PremiumPackService) RemoveItem(packageID, productID string) error {
	pid, err := uuid.Parse(packageID)
	if err != nil {
		return errors.New("invalid package id")
	}
	prodID, err := uuid.Parse(productID)
	if err != nil {
		return errors.New("invalid product id")
	}
	return models.RemovePackageItem(s.db, pid, prodID)
}
