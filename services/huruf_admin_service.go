package services

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"arunika_backend/hurufpath"
	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrLetterNotFound  = errors.New("letter not found")
	ErrVersionNotFound = errors.New("version not found")
	// ErrDraftConflict means the draft changed since the caller loaded it.
	ErrDraftConflict = errors.New("draft was changed by someone else")
	// ErrInvalidOrder means a reorder request didn't list every letter once.
	ErrInvalidOrder = errors.New("order must list every letter exactly once")
)

// HurufAdminService is the backoffice side of Belajar Huruf: drafts, publish,
// versions, visibility, the free letter and order. Every write is audited in
// the same transaction.
type HurufAdminService struct {
	db     *gorm.DB
	assets *AssetService
	audit  *AuditService
}

func NewHurufAdminService(db *gorm.DB, assets *AssetService, audit *AuditService) *HurufAdminService {
	return &HurufAdminService{db: db, assets: assets, audit: audit}
}

// AdminLetterRow is one row of the backoffice letter list.
type AdminLetterRow struct {
	ID                    uuid.UUID `json:"id"`
	Upper                 string    `json:"upper"`
	Lower                 string    `json:"lower"`
	SortOrder             int       `json:"sort_order"`
	IsFree                bool      `json:"is_free"`
	Status                string    `json:"status"`
	HasUnpublishedChanges bool      `json:"has_unpublished_changes"`
	Version               *int      `json:"version"`
	Word                  string    `json:"word"`
	ImageURL              string    `json:"image_url"`
	AudioCount            int       `json:"audio_count"`
	StrokeCount           int       `json:"stroke_count"`
	UpdatedBy             string    `json:"updated_by"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type letterRow struct {
	models.Letter
	Version        *int
	Content        models.JSONB
	UpdatedByEmail *string
}

func (s *HurufAdminService) loadRows(tx *gorm.DB, where string, args ...interface{}) ([]letterRow, error) {
	var rows []letterRow
	q := tx.Table("letters AS l").
		Select("l.*, v.version, v.content, a.email AS updated_by_email").
		Joins("LEFT JOIN letter_versions v ON v.id = l.published_version_id").
		Joins("LEFT JOIN admin_users a ON a.id = l.updated_by").
		Order("l.sort_order ASC")
	if where != "" {
		q = q.Where(where, args...)
	}
	err := q.Scan(&rows).Error
	return rows, err
}

func unpublished(r letterRow) bool {
	return r.Version == nil || canonical(r.Draft) != canonical(r.Content)
}

// List returns every letter for the backoffice table.
func (s *HurufAdminService) List() ([]AdminLetterRow, error) {
	rows, err := s.loadRows(s.db, "")
	if err != nil {
		return nil, err
	}
	drafts := make([]models.LetterContent, len(rows))
	var ids []uuid.UUID
	for i, r := range rows {
		drafts[i], _ = parseContent(r.Draft)
		if id := drafts[i].Kenali.ImageAssetID; id != nil {
			ids = append(ids, *id)
		}
	}
	assets, err := s.assets.Find(s.db, ids)
	if err != nil {
		return nil, err
	}
	out := make([]AdminLetterRow, len(rows))
	for i, r := range rows {
		d := drafts[i]
		out[i] = AdminLetterRow{
			ID: r.ID, Upper: r.Upper, Lower: strings.ToLower(r.Upper),
			SortOrder: r.SortOrder, IsFree: r.IsFree, Status: r.Status,
			HasUnpublishedChanges: unpublished(r),
			Version:               r.Version,
			Word:                  d.Kenali.Word,
			ImageURL:              mediaURL(assets, d.Kenali.ImageAssetID, d.Kenali.ImageURL),
			AudioCount:            audioCount(d),
			StrokeCount:           len(d.Tebalkan.Upper),
			UpdatedBy:             deref(r.UpdatedByEmail),
			UpdatedAt:             r.UpdatedAt,
		}
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// DraftView is a letter's editable state.
type DraftView struct {
	AdminLetterRow
	Draft    models.LetterContent    `json:"draft"`
	DraftRev int                     `json:"draft_rev"`
	Assets   map[uuid.UUID]AssetView `json:"assets"`
}

// GetDraft returns the draft with its referenced assets (for file names,
// sizes and durations in the editor).
func (s *HurufAdminService) GetDraft(id uuid.UUID) (*DraftView, error) {
	return s.draftView(s.db, id)
}

func (s *HurufAdminService) draftView(tx *gorm.DB, id uuid.UUID) (*DraftView, error) {
	rows, err := s.loadRows(tx, "l.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrLetterNotFound
	}
	r := rows[0]
	d, err := parseContent(r.Draft)
	if err != nil {
		return nil, err
	}
	assets, err := s.assets.Find(tx, assetIDs(d))
	if err != nil {
		return nil, err
	}
	return &DraftView{
		AdminLetterRow: AdminLetterRow{
			ID: r.ID, Upper: r.Upper, Lower: strings.ToLower(r.Upper),
			SortOrder: r.SortOrder, IsFree: r.IsFree, Status: r.Status,
			HasUnpublishedChanges: unpublished(r), Version: r.Version,
			Word: d.Kenali.Word, ImageURL: mediaURL(assets, d.Kenali.ImageAssetID, d.Kenali.ImageURL),
			AudioCount: audioCount(d), StrokeCount: len(d.Tebalkan.Upper),
			UpdatedBy: deref(r.UpdatedByEmail), UpdatedAt: r.UpdatedAt,
		},
		Draft:    d,
		DraftRev: r.DraftRev,
		Assets:   assets,
	}, nil
}

// lockLetter loads a letter FOR UPDATE.
func lockLetter(tx *gorm.DB, id uuid.UUID) (*models.Letter, error) {
	var l models.Letter
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&l).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLetterNotFound
	}
	return &l, err
}

// SaveDraft stores draft when rev matches the stored draft_rev. The letter's
// own upper/lower and the grid size are not editable and are forced.
func (s *HurufAdminService) SaveDraft(adminID, id uuid.UUID, draft models.LetterContent, rev int) (*DraftView, error) {
	var out *DraftView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLetter(tx, id)
		if err != nil {
			return err
		}
		if l.DraftRev != rev {
			return ErrDraftConflict
		}
		draft.Upper = l.Upper
		draft.Lower = strings.ToLower(l.Upper)
		draft.Tebalkan.Grid = hurufpath.Grid
		normalizeMedia(&draft)
		before, _ := parseContent(l.Draft)
		raw, err := json.Marshal(draft)
		if err != nil {
			return err
		}
		if err := tx.Model(l).Updates(map[string]interface{}{
			"draft":      string(raw),
			"draft_rev":  gorm.Expr("draft_rev + 1"),
			"updated_by": adminID,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		if err := s.audit.Record(tx, adminID, AuditDraftSave, "letter", id.String(), before, draft); err != nil {
			return err
		}
		out, err = s.draftView(tx, id)
		return err
	})
	return out, err
}

// Publish validates the draft and publishes it as the next version.
func (s *HurufAdminService) Publish(adminID, id uuid.UUID) (*models.LetterVersion, error) {
	var out *models.LetterVersion
	err := s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLetter(tx, id)
		if err != nil {
			return err
		}
		c, err := parseContent(l.Draft)
		if err != nil {
			return err
		}
		assets, err := s.assets.Find(tx, assetIDs(c))
		if err != nil {
			return err
		}
		if errs := validateContent(c, l.Upper, assets); len(errs) > 0 {
			return &ValidationFailedError{Fields: errs}
		}
		v, err := s.newVersion(tx, adminID, l, l.Draft)
		if err != nil {
			return err
		}
		out = v
		return s.audit.Record(tx, adminID, AuditPublish, "letter", id.String(), nil,
			map[string]interface{}{"version": v.Version})
	})
	return out, err
}

// newVersion inserts content as l's next version and makes it live. A
// hidden letter stays hidden.
func (s *HurufAdminService) newVersion(tx *gorm.DB, adminID uuid.UUID, l *models.Letter, content models.JSONB) (*models.LetterVersion, error) {
	var next int
	if err := tx.Model(&models.LetterVersion{}).Where("letter_id = ?", l.ID).
		Select("COALESCE(MAX(version), 0) + 1").Scan(&next).Error; err != nil {
		return nil, err
	}
	v := models.LetterVersion{LetterID: l.ID, Version: next, Content: content, PublishedBy: adminID}
	if err := tx.Create(&v).Error; err != nil {
		return nil, err
	}
	status := models.LetterStatusPublished
	if l.Status == models.LetterStatusHidden {
		status = models.LetterStatusHidden
	}
	err := tx.Model(l).Updates(map[string]interface{}{
		"published_version_id": v.ID,
		"status":               status,
		"updated_by":           adminID,
		"updated_at":           time.Now(),
	}).Error
	return &v, err
}

// VersionView is a version in the history drawer.
type VersionView struct {
	ID          uuid.UUID `json:"id"`
	Version     int       `json:"version"`
	PublishedBy string    `json:"published_by"`
	PublishedAt time.Time `json:"published_at"`
	IsCurrent   bool      `json:"is_current"`
}

// Versions lists a letter's versions, newest first.
func (s *HurufAdminService) Versions(id uuid.UUID) ([]VersionView, error) {
	var l models.Letter
	if err := s.db.Where("id = ?", id).First(&l).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLetterNotFound
		}
		return nil, err
	}
	var out []VersionView
	err := s.db.Table("letter_versions AS v").
		Select("v.id, v.version, a.email AS published_by, v.published_at").
		Joins("JOIN admin_users a ON a.id = v.published_by").
		Where("v.letter_id = ?", id).Order("v.version DESC").Scan(&out).Error
	for i := range out {
		out[i].IsCurrent = l.PublishedVersionID != nil && out[i].ID == *l.PublishedVersionID
	}
	return out, err
}

// Rollback republishes an old version's content as a new version and makes
// it the draft too, so history only grows.
func (s *HurufAdminService) Rollback(adminID, id uuid.UUID, version int) (*models.LetterVersion, error) {
	var out *models.LetterVersion
	err := s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLetter(tx, id)
		if err != nil {
			return err
		}
		var old models.LetterVersion
		if err := tx.Where("letter_id = ? AND version = ?", id, version).First(&old).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrVersionNotFound
			}
			return err
		}
		v, err := s.newVersion(tx, adminID, l, old.Content)
		if err != nil {
			return err
		}
		if err := tx.Model(l).Updates(map[string]interface{}{
			"draft":     string(old.Content),
			"draft_rev": gorm.Expr("draft_rev + 1"),
		}).Error; err != nil {
			return err
		}
		out = v
		return s.audit.Record(tx, adminID, AuditRollback, "letter", id.String(),
			map[string]interface{}{"from_version": version}, map[string]interface{}{"version": v.Version})
	})
	return out, err
}

// SetHidden hides or unhides a letter. Unhiding restores "published" when a
// version exists, "draft" otherwise. Progress rows are never touched.
func (s *HurufAdminService) SetHidden(adminID, id uuid.UUID, hidden bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLetter(tx, id)
		if err != nil {
			return err
		}
		status := models.LetterStatusHidden
		action := AuditHide
		if !hidden {
			action = AuditUnhide
			status = models.LetterStatusDraft
			if l.PublishedVersionID != nil {
				status = models.LetterStatusPublished
			}
		}
		before := l.Status
		if err := tx.Model(l).Updates(map[string]interface{}{
			"status": status, "updated_by": adminID, "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		return s.audit.Record(tx, adminID, action, "letter", id.String(),
			map[string]string{"status": before}, map[string]string{"status": status})
	})
}

// SetFree makes id the only free letter, or, with free false, makes it
// premium again (leaving no free letter if it was the free one).
func (s *HurufAdminService) SetFree(adminID, id uuid.UUID, free bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLetter(tx, id)
		if err != nil {
			return err
		}
		if !free {
			if err := tx.Model(l).Updates(map[string]interface{}{
				"is_free": false, "updated_by": adminID, "updated_at": time.Now(),
			}).Error; err != nil {
				return err
			}
			return s.audit.Record(tx, adminID, AuditUnsetFree, "letter", id.String(),
				map[string]interface{}{"is_free": l.IsFree}, map[string]interface{}{"is_free": false})
		}
		var previous []string
		if err := tx.Model(&models.Letter{}).Where("is_free AND id <> ?", id).
			Pluck("upper", &previous).Error; err != nil {
			return err
		}
		// Clear first: the partial unique index allows only one free row.
		if err := tx.Model(&models.Letter{}).Where("is_free AND id <> ?", id).
			Update("is_free", false).Error; err != nil {
			return err
		}
		if err := tx.Model(l).Updates(map[string]interface{}{
			"is_free": true, "updated_by": adminID, "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		return s.audit.Record(tx, adminID, AuditSetFree, "letter", id.String(),
			map[string]interface{}{"free": previous}, map[string]interface{}{"free": l.Upper})
	})
}

// Reorder sets sort_order 1..n in the order of ids, which must list every
// letter exactly once.
func (s *HurufAdminService) Reorder(adminID uuid.UUID, ids []uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var all []models.Letter
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Order("sort_order ASC").Find(&all).Error; err != nil {
			return err
		}
		if len(ids) != len(all) {
			return ErrInvalidOrder
		}
		known := map[uuid.UUID]bool{}
		before := make([]string, len(all))
		for i, l := range all {
			known[l.ID] = true
			before[i] = l.Upper
		}
		seen := map[uuid.UUID]bool{}
		for _, id := range ids {
			if !known[id] || seen[id] {
				return ErrInvalidOrder
			}
			seen[id] = true
		}
		byID := map[uuid.UUID]string{}
		for _, l := range all {
			byID[l.ID] = l.Upper
		}
		after := make([]string, len(ids))
		for i, id := range ids {
			after[i] = byID[id]
			if err := tx.Model(&models.Letter{}).Where("id = ?", id).
				Update("sort_order", i+1).Error; err != nil {
				return err
			}
		}
		return s.audit.Record(tx, adminID, AuditReorder, "letter", "order",
			map[string]interface{}{"order": before}, map[string]interface{}{"order": after})
	})
}

// Preview resolves an unsaved draft the way the app will see it.
func (s *HurufAdminService) Preview(id uuid.UUID, draft models.LetterContent) (*LetterView, error) {
	var l models.Letter
	if err := s.db.Where("id = ?", id).First(&l).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLetterNotFound
		}
		return nil, err
	}
	draft.Upper = l.Upper
	draft.Lower = strings.ToLower(l.Upper)
	return buildLetterView(s.db, s.assets, l.ID, 0, draft)
}
