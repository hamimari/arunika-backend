package services

import (
	"arunika_backend/models"
	"errors"
	"gorm.io/gorm"
)

type BannerService struct {
	db *gorm.DB
}

func NewBannerService(db *gorm.DB) *BannerService {
	return &BannerService{db: db}
}

// GetActiveBanners returns banners for the mobile app home screen.
func (s *BannerService) GetActiveBanners() ([]models.Banner, error) {
	return models.FindActiveBanners(s.db)
}

// --- Admin methods ---

func (s *BannerService) List(search string, page, perPage int) ([]models.Banner, int64, error) {
	var items []models.Banner
	var total int64
	q := s.db.Model(&models.Banner{}).Where("is_deleted = false")
	if search != "" {
		q = q.Where("title ILIKE ?", "%"+search+"%")
	}
	q.Count(&total)
	err := q.Order("sort_order ASC, created_at DESC").
		Limit(perPage).Offset((page - 1) * perPage).Find(&items).Error
	return items, total, err
}

func (s *BannerService) Get(id string) (*models.Banner, error) {
	var item models.Banner
	err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error
	return &item, err
}

func (s *BannerService) Create(input models.Banner) (*models.Banner, error) {
	if err := s.db.Create(&input).Error; err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *BannerService) Update(id string, input models.Banner) (*models.Banner, error) {
	var item models.Banner
	if err := s.db.Where("id = ? AND is_deleted = false", id).First(&item).Error; err != nil {
		return nil, errors.New("not found")
	}
	if err := s.db.Model(&item).Select(
		"title", "image_url", "link_url", "description", "is_active", "sort_order", "hidden",
	).Updates(input).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *BannerService) Delete(id string) error {
	return s.db.Model(&models.Banner{}).Where("id = ?", id).Update("is_deleted", true).Error
}

func (s *BannerService) ToggleVisibility(id string, hidden bool) error {
	return s.db.Model(&models.Banner{}).Where("id = ?", id).Update("hidden", hidden).Error
}

func (s *BannerService) ToggleActive(id string, active bool) error {
	return s.db.Model(&models.Banner{}).Where("id = ?", id).Update("is_active", active).Error
}
