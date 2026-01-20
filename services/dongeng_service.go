package services

import (
	"arunika_backend/models"
	"gorm.io/gorm"
)

type DongengService struct {
	db *gorm.DB
}

func NewDongengService(db *gorm.DB) *DongengService {
	return &DongengService{db: db}
}

func (s *DongengService) GetFairyTales() ([]models.Dongeng, error) {
	return models.FindAllFairyTales(s.db)
}
