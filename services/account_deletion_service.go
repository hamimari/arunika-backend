package services

import (
	"arunika_backend/models"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AccountDeletionService implements Google Play's account/data deletion
// requirement: personal data the user directly owns is deleted outright,
// while financial ledger rows (orders, payments) that must be retained for
// accounting stay in place, linked to a Parent row that has had its
// personally-identifying fields scrubbed and is flagged IsDeleted so it can
// never authenticate again.
type AccountDeletionService struct {
	db *gorm.DB
}

func NewAccountDeletionService(db *gorm.DB) *AccountDeletionService {
	return &AccountDeletionService{db: db}
}

// DeleteAccount deletes/anonymizes everything owned by userID. Safe to call
// only once — a second call finds no non-deleted parent row and returns
// gorm.ErrRecordNotFound.
func (s *AccountDeletionService) DeleteAccount(userID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var parent models.Parent
		if err := tx.Where("id = ? AND is_deleted = false", userID).First(&parent).Error; err != nil {
			return err
		}

		var childIDs []uuid.UUID
		if err := tx.Model(&models.Children{}).Where("parent_id = ?", userID).Pluck("id", &childIDs).Error; err != nil {
			return fmt.Errorf("list children: %w", err)
		}

		if len(childIDs) > 0 {
			if err := tx.Where("child_id IN ?", childIDs).Delete(&models.GrowthRecord{}).Error; err != nil {
				return fmt.Errorf("delete growth records: %w", err)
			}
		}
		if err := tx.Where("parent_id = ?", userID).Delete(&models.Children{}).Error; err != nil {
			return fmt.Errorf("delete children: %w", err)
		}

		// Personal/behavioral data with no accounting relevance — deleted
		// outright.
		for _, del := range []struct {
			table interface{}
			col   string
		}{
			{&models.UserBadge{}, "user_id"},
			{&models.TracingProgress{}, "user_id"},
			{&models.CountingProgress{}, "user_id"},
			{&models.DongengPlayHistory{}, "user_id"},
			{&models.UserEntitlement{}, "user_id"},
			{&models.UserSubscription{}, "user_id"},
			{&models.Notification{}, "user_id"},
			{&models.FCMToken{}, "user_id"},
			{&models.UserSession{}, "user_id"},
			{&models.RefreshToken{}, "user_id"},
		} {
			if err := tx.Where(del.col+" = ?", userID).Delete(del.table).Error; err != nil {
				return fmt.Errorf("delete %T: %w", del.table, err)
			}
		}

		// Orders/Payments are intentionally left in place (accounting
		// retention) — anonymized by scrubbing the Parent row below rather
		// than deleted, since their user_id is a NOT NULL foreign key.

		placeholder, err := randomPlaceholderEmail()
		if err != nil {
			return fmt.Errorf("generate placeholder: %w", err)
		}
		invalidHash, err := randomUnusablePasswordHash()
		if err != nil {
			return fmt.Errorf("generate password hash: %w", err)
		}

		return tx.Model(&parent).Updates(map[string]interface{}{
			"name":          "Pengguna Terhapus",
			"phone_number":  "",
			"email_address": placeholder,
			"password":      invalidHash,
			"address":       "",
			"city":          "",
			"is_deleted":    true,
		}).Error
	})
}

// randomPlaceholderEmail returns a unique, unreachable address so the
// unique email_address constraint is never violated by repeated deletions,
// while making unmistakably clear (via the .invalid TLD, reserved by
// RFC 2606 for exactly this purpose) that the account is gone.
func randomPlaceholderEmail() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("deleted-%s@arunika.invalid", hex.EncodeToString(b[:])), nil
}

// randomUnusablePasswordHash bcrypt-hashes random bytes so the stored hash
// can never match any real password a user could type.
func randomUnusablePasswordHash() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword(b[:], bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
