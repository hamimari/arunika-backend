package models

import (
	"fmt"
	"gorm.io/gorm"
	"time"
)

type RefreshToken struct {
	BaseModel
	UserId    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func FindUserUserIdAndToken(db *gorm.DB, userId string, token string) (*RefreshToken, error) {
	var refreshToken RefreshToken
	result := db.Where("token = ? and user_id = ?", token, userId).First(&refreshToken)
	if result.Error != nil {
		return nil, result.Error
	}
	return &refreshToken, nil
}

func DeleteByToken(db *gorm.DB, token string) error {
	result := db.Where("token = ?", token).Delete(&RefreshToken{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no token found to delete")
	}
	return nil
}
