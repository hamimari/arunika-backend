package services

import (
	"arunika_backend/models"
	"gorm.io/gorm"
)

type ArService struct {
	db *gorm.DB
}

func NewArService(db *gorm.DB) *ArService {
	return &ArService{db: db}
}

func (s *ArService) GetByID(id string) (*models.ArCards, error) {
	return models.FindCardById(s.db, id)
}
