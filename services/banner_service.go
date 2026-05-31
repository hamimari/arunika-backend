package services

import (
	"arunika_backend/models"
	"gorm.io/gorm"
)

type BannerService struct {
	db *gorm.DB
}

func NewBannerService(db *gorm.DB) *BannerService {
	return &BannerService{db: db}
}

func (s *BannerService) GetActiveBanners() ([]models.Banner, error) {
	return models.FindActiveBanners(s.db)
}
