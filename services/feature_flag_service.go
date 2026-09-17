package services

import (
	"arunika_backend/models"
	"time"

	"gorm.io/gorm"
)

type FeatureFlagService struct {
	db *gorm.DB
}

func NewFeatureFlagService(db *gorm.DB) *FeatureFlagService {
	return &FeatureFlagService{db: db}
}

// List returns every flag ordered by key (admin use).
func (s *FeatureFlagService) List() ([]models.FeatureFlag, error) {
	var flags []models.FeatureFlag
	err := s.db.Order("key ASC").Find(&flags).Error
	return flags, err
}

// EnabledMap returns key -> is_enabled for every flag, the compact shape the
// mobile app consumes.
func (s *FeatureFlagService) EnabledMap() (map[string]bool, error) {
	flags, err := s.List()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(flags))
	for _, f := range flags {
		out[f.Key] = f.IsEnabled
	}
	return out, nil
}

// SetEnabled turns a flag on or off. Returns gorm.ErrRecordNotFound for an
// unknown key — flags are only created by migration, never from the API.
func (s *FeatureFlagService) SetEnabled(key string, enabled bool) (*models.FeatureFlag, error) {
	result := s.db.Model(&models.FeatureFlag{}).
		Where("key = ?", key).
		Updates(map[string]interface{}{"is_enabled": enabled, "updated_at": time.Now()})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var flag models.FeatureFlag
	if err := s.db.Where("key = ?", key).First(&flag).Error; err != nil {
		return nil, err
	}
	return &flag, nil
}
