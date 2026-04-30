package services

import (
	"arunika_backend/models"
	"context"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"os"
	"time"
)

// AdminAuthService handles authentication for admin users.
type AdminAuthService struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewAdminAuthService(db *gorm.DB, redis *redis.Client) *AdminAuthService {
	return &AdminAuthService{db: db, redis: redis}
}

// Login validates credentials and returns (accessToken, refreshToken, error).
func (s *AdminAuthService) Login(email, password string) (string, string, error) {
	admin, err := models.FindAdminByEmail(s.db, email)
	if err != nil {
		return "", "", errors.New("invalid credentials")
	}
	if !models.CheckAdminPassword(admin.PasswordHash, password) {
		return "", "", errors.New("invalid credentials")
	}
	return s.generateTokens(admin.ID.String(), admin.Email)
}

// RefreshToken validates an admin refresh token and issues a new access token.
func (s *AdminAuthService) RefreshToken(adminID, refreshToken string) (string, error) {
	// Validate stored refresh token
	var stored models.RefreshToken
	err := s.db.Where("user_id = ? AND token = ? AND is_deleted = false", adminID, refreshToken).First(&stored).Error
	if err != nil {
		return "", errors.New("token not found")
	}
	if time.Now().After(stored.ExpiresAt) {
		return "", errors.New("token expired")
	}
	accessToken, _, err := s.generateAdminJWT(adminID, stored.UserId)
	return accessToken, err
}

// Logout blacklists the JTI in Redis and marks the refresh token deleted.
func (s *AdminAuthService) Logout(jti, refreshToken string, expiry time.Time) error {
	ctx := context.Background()
	ttl := time.Until(expiry)
	if ttl > 0 {
		s.redis.Set(ctx, "blacklist:"+jti, "revoked", ttl)
	}
	s.db.Where("token = ?", refreshToken).Delete(&models.RefreshToken{})
	return nil
}

// HashPassword creates a bcrypt hash at cost 12.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	return string(b), err
}

// --- internal helpers ---

func (s *AdminAuthService) generateTokens(adminID, email string) (string, string, error) {
	accessToken, refreshToken, err := s.generateAdminJWT(adminID, email)
	if err != nil {
		return "", "", err
	}

	// Persist refresh token (reuse refresh_tokens table, same structure).
	rt := models.RefreshToken{
		UserId:    adminID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := s.db.Create(&rt).Error; err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func (s *AdminAuthService) generateAdminJWT(adminID, email string) (string, string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", "", fmt.Errorf("JWT_SECRET not set")
	}
	refreshToken := uuid.NewString()
	jti := uuid.NewString()

	claims := jwt.MapClaims{
		"sub":           adminID,
		"email":         email,
		"role":          "admin",
		"refresh_token": refreshToken,
		"jti":           jti,
		"exp":           time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}
	return tokenString, refreshToken, nil
}
