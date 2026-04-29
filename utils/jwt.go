package utils

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"os"
	"time"
)

// GenerateJWT creates a signed access token and a refresh token UUID.
// The JWT_SECRET is read at call-time (not at package init) so that it is
// always available regardless of when godotenv.Load() was called.
func GenerateJWT(userID string, email string) (string, string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", "", fmt.Errorf("JWT_SECRET is not set")
	}

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
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, refreshToken, nil
}
