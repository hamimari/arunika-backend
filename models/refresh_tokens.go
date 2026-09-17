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

// FindRefreshToken looks a refresh token up by its value alone. Refreshing a
// session presents only this token — the access token it was issued with has
// usually expired by then — so the user it belongs to comes from the row.
func FindRefreshToken(db *gorm.DB, token string) (*RefreshToken, error) {
	var refreshToken RefreshToken
	result := db.Where("token = ?", token).First(&refreshToken)
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

// DeleteAllRefreshTokensByUserId revokes every session for a user — used
// after a password reset so a leaked/compromised refresh token stops
// working immediately instead of remaining valid for its full lifetime.
func DeleteAllRefreshTokensByUserId(db *gorm.DB, userId string) error {
	return db.Where("user_id = ?", userId).Delete(&RefreshToken{}).Error
}
