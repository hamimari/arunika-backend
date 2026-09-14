package models

import "gorm.io/gorm"

type Feature struct {
	BaseModel
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string
	IsActive    bool
}

func FindFeatureByCode(db *gorm.DB, code string) (*Feature, error) {
	var feature Feature
	if err := db.Where("code = ?", code).First(&feature).Error; err != nil {
		return nil, err
	}
	return &feature, nil
}
