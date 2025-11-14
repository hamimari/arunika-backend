package utils

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"os"
	"time"
)

var secretKey = os.Getenv("JWT_SECRET")

func GenerateJWT(userID string, email string) (string, string, error) {
	refreshToken := uuid.NewString()
	jti := uuid.NewString()

	claims := jwt.MapClaims{
		"sub":           userID,
		"email":         email,
		"refresh_token": refreshToken,
		"jti":           jti,
		"exp":           time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secretKey))
	return tokenString, refreshToken, nil
}
