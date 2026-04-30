package services

import (
	"arunika_backend/models"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type AdminUserService struct {
	db *gorm.DB
}

func NewAdminUserService(db *gorm.DB) *AdminUserService {
	return &AdminUserService{db: db}
}

type AdminUserListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	City      string `json:"city"`
	Status    string `json:"subscription_status"`
	CreatedAt string `json:"created_at"`
}

type AdminUserDetail struct {
	models.Parent
	Subscription *models.UserSubscription  `json:"subscription,omitempty"`
	Payments     []models.UserSubscription `json:"payment_history"`
}

// ListUsers returns paginated users with optional name/email search.
func (s *AdminUserService) ListUsers(search string, page, perPage int) ([]models.Parent, int64, error) {
	var users []models.Parent
	var total int64
	q := s.db.Model(&models.Parent{}).Where("is_deleted = false")
	if search != "" {
		q = q.Where("name ILIKE ? OR email_address ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	q.Count(&total)
	err := q.Limit(perPage).Offset((page - 1) * perPage).Order("created_at DESC").Find(&users).Error
	return users, total, err
}

// GetUserDetail returns user profile + subscription record.
func (s *AdminUserService) GetUserDetail(id string) (*models.Parent, *models.UserSubscription, error) {
	var user models.Parent
	if err := s.db.Where("id = ? AND is_deleted = false", id).First(&user).Error; err != nil {
		return nil, nil, err
	}
	var sub models.UserSubscription
	s.db.Where("user_id = ?", id).First(&sub)
	return &user, &sub, nil
}

// GrantPremium sets the user's subscription to premium for durationDays.
// If durationDays is 0, it grants premium with no expiry.
// If no subscription record exists yet (e.g. user never purchased), one is created.
func (s *AdminUserService) GrantPremium(userID string, durationDays int) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}

	var sub models.UserSubscription
	result := s.db.Where("user_id = ?", userID).First(&sub)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}
		// No subscription record — create one.
		sub = models.UserSubscription{
			UserID: uid,
			Status: "premium",
		}
		if durationDays > 0 {
			expiry := time.Now().Add(time.Duration(durationDays) * 24 * time.Hour)
			sub.ExpiresAt = &expiry
		}
		return s.db.Create(&sub).Error
	}

	updates := map[string]interface{}{
		"status":     "premium",
		"updated_at": time.Now(),
	}
	if durationDays > 0 {
		expiry := time.Now().Add(time.Duration(durationDays) * 24 * time.Hour)
		updates["expires_at"] = expiry
	}
	return s.db.Model(&sub).Updates(updates).Error
}

// RevokePremium sets the user's subscription back to free immediately.
// If no subscription record exists, it is a no-op (nothing to revoke).
func (s *AdminUserService) RevokePremium(userID string) error {
	var sub models.UserSubscription
	result := s.db.Where("user_id = ?", userID).First(&sub)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil // nothing to revoke
		}
		return result.Error
	}
	now := time.Now()
	return s.db.Model(&sub).Updates(map[string]interface{}{
		"status":     "free",
		"expires_at": now,
		"updated_at": now,
	}).Error
}
