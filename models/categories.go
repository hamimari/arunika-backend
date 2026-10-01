package models

import "gorm.io/gorm"

type Categories struct {
	BaseModel
	Name     string `json:"name"`
	ImageUrl string `json:"image_url"`
	Hidden   bool   `gorm:"column:hidden;default:false" json:"hidden"`
}

func FindAll(db *gorm.DB) ([]Categories, error) {
	var categories []Categories

	result := db.Where("is_deleted = ? AND hidden = ?", false, false).Find(&categories)
	if result.Error != nil {
		return nil, result.Error
	}
	return categories, nil

}
