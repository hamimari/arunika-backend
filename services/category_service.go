package services

import (
	"arunika_backend/models"
	"gorm.io/gorm"
)

type CategoryService struct {
	db *gorm.DB
}

func NewCategoryService(db *gorm.DB) *CategoryService {
	return &CategoryService{db: db}
}

func (s *CategoryService) GetCategories() ([]models.Categories, error) {
	return models.FindAll(s.db)
}
