package models

import (
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"time"
)

type AdminUser struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null"                            json:"email"`
	PasswordHash string    `gorm:"not null"                                        json:"-"`
	Role         string    `gorm:"not null;default:publisher"                      json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	IsDeleted    bool      `gorm:"default:false"                                   json:"-"`
}

func (AdminUser) TableName() string { return "admin_users" }

// Content roles. Editors edit drafts and upload assets; publishers can also
// publish, hide, reorder, roll back and manage roles.
const (
	AdminRoleEditor    = "editor"
	AdminRolePublisher = "publisher"
)

func FindAdminByEmail(db *gorm.DB, email string) (*AdminUser, error) {
	var admin AdminUser
	err := db.Where("email = ? AND is_deleted = false", email).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func CheckAdminPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
