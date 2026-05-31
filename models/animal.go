package models

import (
	"gorm.io/gorm"
	"time"
)

type Animal struct {
	ID         string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name       string    `gorm:"type:varchar(255);not null"                     json:"name"`
	Emoji      string    `gorm:"type:varchar(20);not null;default:''"           json:"emoji"`
	Category   string    `gorm:"type:varchar(50);not null;default:'hutan'"      json:"category"`
	ImageURL   string    `gorm:"type:text;not null;default:''"                  json:"image_url"`
	BgColor    string    `gorm:"type:varchar(20);not null;default:'#FFF3E0'"    json:"bg_color"`
	Fact       string    `gorm:"type:text;not null;default:''"                  json:"fact"`
	IsUnlocked bool      `gorm:"not null;default:false"                         json:"is_unlocked"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	IsDeleted  bool      `gorm:"not null;default:false"                         json:"is_deleted"`
}

func (Animal) TableName() string {
	return "animals"
}

// FindAllAnimals returns all non-deleted animals, optionally filtered by category.
func FindAllAnimals(db *gorm.DB, category string) ([]Animal, error) {
	var animals []Animal
	q := db.Where("is_deleted = false")
	if category != "" && category != "all" {
		q = q.Where("category = ?", category)
	}
	result := q.Order("name asc").Find(&animals)
	return animals, result.Error
}
