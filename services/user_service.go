package services

import (
	"arunika_backend/models"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) GetUserByID(id string) (*models.Parent, error) {
	var user models.Parent
	if err := s.db.Preload("Children").First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
