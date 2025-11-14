package models

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Parent struct {
	BaseModel
	Name         string     `json:"name"`
	PhoneNumber  string     `json:"phone_number"`
	EmailAddress string     `gorm:"unique" json:"email_address"`
	Password     string     `json:"password"`
	Address      string     `json:"address"`
	City         string     `json:"city"`
	Children     []Children `json:"children" gorm:"foreignKey:ParentId"`
}

func FindUserByEmail(db *gorm.DB, email string) (*Parent, error) {
	var user Parent
	result := db.Where("email_address = ?", email).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func CheckPassword(storedHash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
	return err == nil
}
