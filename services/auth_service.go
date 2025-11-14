package services

import (
	"arunika_backend/models"
	"arunika_backend/utils"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"os"
	"time"
)

type AuthService struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewAuthService(db *gorm.DB, redis *redis.Client) *AuthService {
	return &AuthService{db: db, redis: redis}
}

func (s *AuthService) GenerateJwtToken(userId, email string) (string, string, error) {
	token, refreshToken, err := utils.GenerateJWT(userId, email)

	refreshTokenExpiresAt := time.Now().Add(7 * 24 * time.Hour)
	refreshTokenEntity := models.RefreshToken{
		UserId:    userId,
		Token:     refreshToken,
		ExpiresAt: refreshTokenExpiresAt,
	}

	err = s.db.Create(&refreshTokenEntity).Error
	if err != nil {
		return "", "", err
	}

	return token, refreshToken, err
}

func (s *AuthService) Signup(request models.Parent) (models.Parent, error) {
	user, _ := models.FindUserByEmail(s.db, request.EmailAddress)
	if user != nil {
		return models.Parent{}, errors.New("email address already taken")
	}

	if err := s.db.Create(&request).Error; err != nil {
		return models.Parent{}, err
	}

	return request, nil
}

func (s *AuthService) SendOtp(request models.Parent) (models.Parent, error) {
	user, _ := models.FindUserByEmail(s.db, request.EmailAddress)
	err := SendOTPEmail(user.EmailAddress)
	if err != nil {
		return models.Parent{}, errors.New("error when sending the otp")
	}

	return request, nil
}

func (s *AuthService) ValidateRefreshToken(userId string, refreshToken string) (string, error) {
	refreshTokenEntity, err := models.FindUserUserIdAndToken(s.db, userId, refreshToken)
	if err != nil {
		return "", errors.New("token not found")
	}

	if time.Now().After(refreshTokenEntity.ExpiresAt) {
		return "", errors.New("token expired")
	}

	return refreshTokenEntity.UserId, nil
}

func (s *AuthService) ValidateCredentials(email string, password string) (*models.Parent, error) {
	user, _ := models.FindUserByEmail(s.db, email)
	if user == nil || !models.CheckPassword(user.Password, password) {
		return nil, errors.New("invalid credential")
	}
	return user, nil
}

func (s *AuthService) Logout(ctx *gin.Context, token string, jti string, exp time.Time) error {
	err := models.DeleteByToken(s.db, token)
	if err != nil {
		return errors.New("token not found")
	}

	ttl := time.Until(exp)
	if ttl <= 0 {
		ttl = time.Minute * 15
	}
	return s.redis.Set(ctx, "blacklist:"+jti, "revoked", ttl).Err()
}

func (s *AuthService) ForgotPassword(email string) error {
	var user models.Parent
	if err := s.db.Where("email_address = ?", email).First(&user).Error; err != nil {
		return errors.New("user not found")
	}

	resetToken := uuid.NewString()
	reset := models.PasswordResetToken{
		UserID:    user.ID,
		Token:     resetToken,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	if err := s.db.Create(&reset).Error; err != nil {
		return err
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", os.Getenv("APP_DOMAIN"), resetToken)
	subject := "Password Reset Request"
	body := fmt.Sprintf("Hi %s,\n\nClick the link below to reset your password:\n\n%s\n\nThis link will expire in 15 minutes.\n\nBest,\nYour App Team",
		user.Name, resetLink)

	return utils.SendEmail(user.EmailAddress, subject, body)
}

func (s *AuthService) ResetPassword(token, newPassword string) error {
	reset, err := s.VerifyResetToken(token)
	if err != nil {
		return err
	}

	var user models.Parent
	if err := s.db.First(&user, reset.UserID).Error; err != nil {
		return errors.New("user not found")
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), 14)
	if err != nil {
		return err
	}
	hashed := string(bytes)

	user.Password = hashed
	if err := s.db.Save(&user).Error; err != nil {
		return err
	}

	s.db.Delete(&reset)
	return nil
}

func (s *AuthService) VerifyResetToken(token string) (*models.PasswordResetToken, error) {
	var reset models.PasswordResetToken
	if err := s.db.Where("token = ?", token).First(&reset).Error; err != nil {
		return nil, errors.New("invalid token")
	}

	if time.Now().After(reset.ExpiresAt) {
		return nil, errors.New("token expired")
	}
	return &reset, nil
}
