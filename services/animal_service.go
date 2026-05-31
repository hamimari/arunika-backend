package services

import (
	"arunika_backend/models"
	"gorm.io/gorm"
)

type AnimalService struct {
	db *gorm.DB
}

func NewAnimalService(db *gorm.DB) *AnimalService {
	return &AnimalService{db: db}
}

// GetAnimals returns all animals, optionally filtered by category.
func (s *AnimalService) GetAnimals(category string) ([]models.Animal, error) {
	return models.FindAllAnimals(s.db, category)
}
