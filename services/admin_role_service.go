package services

import (
	"errors"

	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrAdminNotFound = errors.New("admin not found")
	ErrInvalidRole   = errors.New("invalid role")
	// ErrLastPublisher stops a role change that would leave nobody able to
	// publish or manage roles.
	ErrLastPublisher = errors.New("at least one publisher is required")
)

// AdminRoleService lists admins and changes their content role.
type AdminRoleService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewAdminRoleService(db *gorm.DB, audit *AuditService) *AdminRoleService {
	return &AdminRoleService{db: db, audit: audit}
}

type AdminView struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Role  string    `json:"role"`
}

func (s *AdminRoleService) List() ([]AdminView, error) {
	var out []AdminView
	err := s.db.Model(&models.AdminUser{}).
		Where("is_deleted = false").Order("email ASC").
		Select("id, email, role").Scan(&out).Error
	return out, err
}

// SetRole changes targetID's role, by actorID.
func (s *AdminRoleService) SetRole(actorID, targetID uuid.UUID, role string) (*AdminView, error) {
	if role != models.AdminRoleEditor && role != models.AdminRolePublisher {
		return nil, ErrInvalidRole
	}
	var out *AdminView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Lock every publisher row so two concurrent demotions can't both
		// pass the last-publisher check.
		var publishers []uuid.UUID
		if err := tx.Raw(`SELECT id FROM admin_users
			WHERE role = 'publisher' AND is_deleted = false FOR UPDATE`).
			Scan(&publishers).Error; err != nil {
			return err
		}
		var admin models.AdminUser
		if err := tx.Where("id = ? AND is_deleted = false", targetID).First(&admin).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAdminNotFound
			}
			return err
		}
		if admin.Role == role {
			out = &AdminView{ID: admin.ID, Email: admin.Email, Role: role}
			return nil
		}
		if admin.Role == models.AdminRolePublisher && len(publishers) <= 1 {
			return ErrLastPublisher
		}
		before := admin.Role
		if err := tx.Model(&admin).Update("role", role).Error; err != nil {
			return err
		}
		if err := s.audit.Record(tx, actorID, AuditRoleChange, "admin", admin.ID.String(),
			map[string]string{"role": before}, map[string]string{"role": role}); err != nil {
			return err
		}
		out = &AdminView{ID: admin.ID, Email: admin.Email, Role: role}
		return nil
	})
	return out, err
}
