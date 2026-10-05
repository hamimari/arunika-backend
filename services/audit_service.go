package services

import (
	"errors"
	"time"

	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Audit actions.
const (
	AuditDraftSave  = "draft.save"
	AuditPublish    = "publish"
	AuditRollback   = "rollback"
	AuditHide       = "hide"
	AuditUnhide     = "unhide"
	AuditSetFree    = "set_free"
	AuditUnsetFree  = "unset_free"
	AuditReorder    = "reorder"
	AuditUpload     = "upload"
	AuditRoleChange = "role.change"
)

// draftSaveCoalesceWindow merges an admin's back-to-back autosaves of one
// entity into a single audit row, so the history stays readable.
const draftSaveCoalesceWindow = 10 * time.Minute

// AuditService writes and reads admin_audit_log. Record must be called with
// the caller's transaction so the action and its audit row commit together.
type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// Record appends an audit entry, in tx when given (nil uses the service's
// own connection). A draft.save following one by the same admin
// on the same entity within draftSaveCoalesceWindow updates that entry's
// `after` instead of adding a row.
func (s *AuditService) Record(tx *gorm.DB, adminID uuid.UUID, action, entityType, entityID string, before, after interface{}) error {
	if tx == nil {
		tx = s.db
	}
	if action == AuditDraftSave {
		var last models.AdminAuditLog
		err := tx.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
			Order("created_at DESC, id DESC").First(&last).Error
		if err == nil && last.Action == AuditDraftSave && last.AdminID == adminID &&
			time.Since(last.CreatedAt) < draftSaveCoalesceWindow {
			return tx.Model(&last).Update("after", models.ToJSONB(after)).Error
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return tx.Create(&models.AdminAuditLog{
		AdminID:    adminID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Before:     models.ToJSONB(before),
		After:      models.ToJSONB(after),
	}).Error
}

// AuditEntry is an audit row with the acting admin's email.
type AuditEntry struct {
	models.AdminAuditLog
	AdminEmail string `json:"admin_email"`
}

// List returns entries, newest first, optionally filtered by entity.
func (s *AuditService) List(entityType, entityID string, page, perPage int) ([]AuditEntry, int64, error) {
	q := s.db.Table("admin_audit_log AS l").
		Joins("JOIN admin_users a ON a.id = l.admin_id")
	if entityType != "" {
		q = q.Where("l.entity_type = ?", entityType)
	}
	if entityID != "" {
		q = q.Where("l.entity_id = ?", entityID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []AuditEntry
	err := q.Select("l.*, a.email AS admin_email").
		Order("l.created_at DESC, l.id DESC").
		Limit(perPage).Offset((page - 1) * perPage).
		Scan(&out).Error
	return out, total, err
}
