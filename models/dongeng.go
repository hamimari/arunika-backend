package models

import "gorm.io/gorm"

type Dongeng struct {
	BaseModel
	Title      string  `json:"title"`
	AgeStart   float32 `json:"age_start"`
	AgeEnd     float32 `json:"age_end"`
	ImageUrl   string  `json:"image_url"`
	IsFree     bool    `json:"is_free"`
	CategoryId string  `json:"category_id"`
}

func FindAllFairyTales(db *gorm.DB) ([]Dongeng, error) {
	var fairyTales []Dongeng

	result := db.Where("is_deleted = ?", false).Find(&fairyTales)
	if result.Error != nil {
		return nil, result.Error
	}
	return fairyTales, nil
}
