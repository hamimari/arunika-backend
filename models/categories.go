package models

import "gorm.io/gorm"

type Categories struct {
	BaseModel
	Name     string `json:"name"`
	ImageUrl string `json:"image_url"`
}

func FindAll(db *gorm.DB) ([]Categories, error) {
	var categories []Categories

	result := db.Where("is_deleted = ?", false).Find(&categories)
	if result.Error != nil {
		return nil, result.Error
	}
	return categories, nil

}
