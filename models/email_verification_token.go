package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EmailVerificationToken proves control of the email address on an account.
//
// Token stores a SHA-256 hash of the value emailed to the user (see
// utils.HashToken), never the raw value — so reading this table alone never
// yields a directly usable verification link.
//
// Lifetime is 24h rather than the 15 minutes used for password resets: a
// reset token is a live credential for changing a password, while this one
// only asserts mailbox control, and people open mail hours later. A short
// window here would generate resend traffic for no security gain.
type EmailVerificationToken struct {
	BaseModel
	UserID    uuid.UUID `gorm:"index"`
	Token     string    `gorm:"uniqueIndex"`
	ExpiresAt time.Time
}

// EmailVerificationTokenLifetime is how long an emailed verification link
// stays usable.
const EmailVerificationTokenLifetime = 24 * time.Hour

// CreateEmailVerificationToken stores a new hashed token for userID, removing
// any earlier ones first so that only the most recently emailed link works.
func CreateEmailVerificationToken(db *gorm.DB, userID uuid.UUID, hashedToken string, expiresAt time.Time) error {
	if err := DeleteEmailVerificationTokensForUser(db, userID); err != nil {
		return err
	}
	return db.Create(&EmailVerificationToken{
		UserID:    userID,
		Token:     hashedToken,
		ExpiresAt: expiresAt,
	}).Error
}

// FindEmailVerificationToken looks up a token by its hash.
func FindEmailVerificationToken(db *gorm.DB, hashedToken string) (*EmailVerificationToken, error) {
	var token EmailVerificationToken
	if err := db.Where("token = ?", hashedToken).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// DeleteEmailVerificationTokensForUser removes every outstanding token for a
// user — used both to supersede on reissue and to consume on success.
func DeleteEmailVerificationTokensForUser(db *gorm.DB, userID uuid.UUID) error {
	return db.Unscoped().Where("user_id = ?", userID).Delete(&EmailVerificationToken{}).Error
}

// MarkEmailVerified records that the user has proven control of their address.
func MarkEmailVerified(db *gorm.DB, userID uuid.UUID) error {
	return db.Model(&Parent{}).Where("id = ?", userID).Update("email_verified", true).Error
}
