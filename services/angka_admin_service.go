package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"arunika_backend/angkagen"
	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Audit entity types and the action Huruf didn't need.
const (
	AuditEntityAngkaObject = "angka_object"
	AuditEntityAngkaNumber = "angka_number"
	AuditEntityAngkaLevel  = "angka_level"
	AuditDelete            = "delete"
)

// AngkaAdminService is the backoffice side of Belajar Angka: the object
// library, the Kenal Angka numbers and the Hitung Benda levels. Every write
// is audited in the same transaction.
type AngkaAdminService struct {
	db     *gorm.DB
	assets *AssetService
	audit  *AuditService
}

func NewAngkaAdminService(db *gorm.DB, assets *AssetService, audit *AuditService) *AngkaAdminService {
	return &AngkaAdminService{db: db, assets: assets, audit: audit}
}

// adminStatus is what the backoffice tables show for an item.
type adminStatus struct {
	Status                string    `json:"status"`
	HasUnpublishedChanges bool      `json:"has_unpublished_changes"`
	UpdatedBy             string    `json:"updated_by"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func (s *AngkaAdminService) adminEmails(ids ...*uuid.UUID) (map[uuid.UUID]string, error) {
	var want []uuid.UUID
	for _, id := range ids {
		if id != nil {
			want = append(want, *id)
		}
	}
	out := map[uuid.UUID]string{}
	if len(want) == 0 {
		return out, nil
	}
	var rows []models.AdminUser
	if err := s.db.Where("id IN ?", want).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.Email
	}
	return out, nil
}

func emailOf(m map[uuid.UUID]string, id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return m[*id]
}

// levelRefs maps each object id to the levels whose draft or any version
// references it.
func levelRefs(tx *gorm.DB) (map[uuid.UUID]map[uuid.UUID]bool, error) {
	refs := map[uuid.UUID]map[uuid.UUID]bool{}
	mark := func(level uuid.UUID, raw []byte) {
		for _, o := range parseLevel(raw).ObjectIDs {
			if refs[o] == nil {
				refs[o] = map[uuid.UUID]bool{}
			}
			refs[o][level] = true
		}
	}
	var levels []models.AngkaLevel
	if err := tx.Find(&levels).Error; err != nil {
		return nil, err
	}
	for _, l := range levels {
		mark(l.ID, l.Draft)
	}
	var versions []models.AngkaLevelVersion
	if err := tx.Select("level_id, content").Find(&versions).Error; err != nil {
		return nil, err
	}
	for _, v := range versions {
		mark(v.LevelID, v.Content)
	}
	return refs, nil
}

// ─── Objects ────────────────────────────────────────────────────────────────

// AdminObjectRow is one card of the Pustaka benda tab (from the draft).
type AdminObjectRow struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	QuestionText     string    `json:"question_text"`
	ImageURL         string    `json:"image_url"`
	QuestionAudioURL string    `json:"question_audio_url"`
	UsedIn           int       `json:"used_in"`
	adminStatus
}

// AdminObjectDraft is an object's editable state.
type AdminObjectDraft struct {
	AdminObjectRow
	Draft    models.AngkaObjectContent `json:"draft"`
	DraftRev int                       `json:"draft_rev"`
	Assets   map[uuid.UUID]AssetView   `json:"assets"`
}

func (s *AngkaAdminService) objectRows(tx *gorm.DB, where string, args ...interface{}) ([]AdminObjectRow, []models.AngkaObject, error) {
	var objs []models.AngkaObject
	q := tx.Order("created_at ASC, id ASC")
	if where != "" {
		q = q.Where(where, args...)
	}
	if err := q.Find(&objs).Error; err != nil {
		return nil, nil, err
	}
	refs, err := levelRefs(tx)
	if err != nil {
		return nil, nil, err
	}
	drafts := make([]models.AngkaObjectContent, len(objs))
	var ids []uuid.UUID
	var by []*uuid.UUID
	for i, o := range objs {
		drafts[i] = parseObject(o.Draft)
		ids = append(ids, objectAssetIDs(drafts[i])...)
		by = append(by, o.UpdatedBy)
	}
	assets, err := s.assets.Find(tx, ids)
	if err != nil {
		return nil, nil, err
	}
	emails, err := s.adminEmails(by...)
	if err != nil {
		return nil, nil, err
	}
	out := make([]AdminObjectRow, len(objs))
	for i, o := range objs {
		d := drafts[i]
		out[i] = AdminObjectRow{
			ID: o.ID, Name: d.Name, QuestionText: d.QuestionText,
			ImageURL:         mediaURL(assets, d.ImageAssetID, d.ImageURL),
			QuestionAudioURL: mediaURL(assets, d.QuestionAudioID, d.QuestionAudioURL),
			UsedIn:           len(refs[o.ID]),
			adminStatus: adminStatus{
				Status:                o.Status,
				HasUnpublishedChanges: !sameJSON[models.AngkaObjectContent](o.Draft, o.Content),
				UpdatedBy:             emailOf(emails, o.UpdatedBy),
				UpdatedAt:             o.UpdatedAt,
			},
		}
	}
	return out, objs, nil
}

// ListObjects returns the object library.
func (s *AngkaAdminService) ListObjects() ([]AdminObjectRow, error) {
	rows, _, err := s.objectRows(s.db, "")
	return rows, err
}

// GetObjectDraft returns an object's draft and its assets.
func (s *AngkaAdminService) GetObjectDraft(id uuid.UUID) (*AdminObjectDraft, error) {
	return s.objectDraft(s.db, id)
}

func (s *AngkaAdminService) objectDraft(tx *gorm.DB, id uuid.UUID) (*AdminObjectDraft, error) {
	rows, objs, err := s.objectRows(tx, "id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrAngkaObjectNotFound
	}
	d := parseObject(objs[0].Draft)
	assets, err := s.assets.Find(tx, objectAssetIDs(d))
	if err != nil {
		return nil, err
	}
	return &AdminObjectDraft{AdminObjectRow: rows[0], Draft: d, DraftRev: objs[0].DraftRev, Assets: assets}, nil
}

// CreateObject adds a draft object named name.
func (s *AngkaAdminService) CreateObject(adminID uuid.UUID, name string) (*AdminObjectDraft, error) {
	var out *AdminObjectDraft
	err := s.db.Transaction(func(tx *gorm.DB) error {
		c := models.AngkaObjectContent{Name: name}
		normalizeObject(&c)
		o := models.AngkaObject{Draft: models.ToJSONB(c), UpdatedBy: &adminID}
		if err := tx.Create(&o).Error; err != nil {
			return err
		}
		if err := s.audit.Record(tx, adminID, AuditDraftSave, AuditEntityAngkaObject, o.ID.String(), nil, c); err != nil {
			return err
		}
		var err error
		out, err = s.objectDraft(tx, o.ID)
		return err
	})
	return out, err
}

func lockObject(tx *gorm.DB, id uuid.UUID) (*models.AngkaObject, error) {
	var o models.AngkaObject
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAngkaObjectNotFound
	}
	return &o, err
}

// SaveObjectDraft stores draft when rev matches.
func (s *AngkaAdminService) SaveObjectDraft(adminID, id uuid.UUID, draft models.AngkaObjectContent, rev int) (*AdminObjectDraft, error) {
	var out *AdminObjectDraft
	err := s.db.Transaction(func(tx *gorm.DB) error {
		o, err := lockObject(tx, id)
		if err != nil {
			return err
		}
		if o.DraftRev != rev {
			return ErrDraftConflict
		}
		normalizeObject(&draft)
		if err := saveDraft(tx, o, adminID, draft); err != nil {
			return err
		}
		if err := s.audit.Record(tx, adminID, AuditDraftSave, AuditEntityAngkaObject, id.String(), parseObject(o.Draft), draft); err != nil {
			return err
		}
		out, err = s.objectDraft(tx, id)
		return err
	})
	return out, err
}

// saveDraft writes a new draft on model and bumps draft_rev.
func saveDraft(tx *gorm.DB, model interface{}, adminID uuid.UUID, draft interface{}) error {
	raw, err := json.Marshal(draft)
	if err != nil {
		return err
	}
	return tx.Model(model).Updates(map[string]interface{}{
		"draft":      string(raw),
		"draft_rev":  gorm.Expr("draft_rev + 1"),
		"updated_by": adminID,
		"updated_at": time.Now(),
	}).Error
}

// publishCopy makes the draft the published copy. A hidden item stays hidden.
func publishCopy(tx *gorm.DB, model interface{}, status string, draft []byte, adminID uuid.UUID) error {
	if status != models.AngkaStatusHidden {
		status = models.AngkaStatusPublished
	}
	return tx.Model(model).Updates(map[string]interface{}{
		"content":    string(draft),
		"status":     status,
		"updated_by": adminID,
		"updated_at": time.Now(),
	}).Error
}

// PublishObject validates the draft and makes it the published copy.
func (s *AngkaAdminService) PublishObject(adminID, id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		o, err := lockObject(tx, id)
		if err != nil {
			return err
		}
		c := parseObject(o.Draft)
		assets, err := s.assets.Find(tx, objectAssetIDs(c))
		if err != nil {
			return err
		}
		if errs := validateObject(c, assets); len(errs) > 0 {
			return &ValidationFailedError{Fields: errs}
		}
		if err := publishCopy(tx, o, o.Status, o.Draft, adminID); err != nil {
			return err
		}
		return s.audit.Record(tx, adminID, AuditPublish, AuditEntityAngkaObject, id.String(), parseObject(o.Content), c)
	})
}

// SetObjectHidden hides or unhides an object. Hiding is refused when a
// published, visible level would be left with no visible object.
func (s *AngkaAdminService) SetObjectHidden(adminID, id uuid.UUID, hidden bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		o, err := lockObject(tx, id)
		if err != nil {
			return err
		}
		if hidden {
			if err := s.checkNotLastObject(tx, id); err != nil {
				return err
			}
		}
		return s.setStatus(tx, o, AuditEntityAngkaObject, id.String(), o.Status, len(o.Content) > 0, hidden, adminID)
	})
}

// setStatus hides, or unhides back to published (or draft when never
// published), and audits it.
func (s *AngkaAdminService) setStatus(tx *gorm.DB, model interface{}, entity, entityID, before string,
	published, hidden bool, adminID uuid.UUID) error {
	status, action := models.AngkaStatusHidden, AuditHide
	if !hidden {
		action = AuditUnhide
		status = models.AngkaStatusDraft
		if published {
			status = models.AngkaStatusPublished
		}
	}
	if err := tx.Model(model).Updates(map[string]interface{}{
		"status": status, "updated_by": adminID, "updated_at": time.Now(),
	}).Error; err != nil {
		return err
	}
	return s.audit.Record(tx, adminID, action, entity, entityID,
		map[string]string{"status": before}, map[string]string{"status": status})
}

func (s *AngkaAdminService) checkNotLastObject(tx *gorm.DB, id uuid.UUID) error {
	var rows []struct {
		Content models.JSONB
	}
	if err := tx.Table("angka_levels AS l").Select("v.content").
		Joins("JOIN angka_level_versions v ON v.id = l.published_version_id").
		Where("l.status = ?", models.AngkaStatusPublished).Scan(&rows).Error; err != nil {
		return err
	}
	var objs []models.AngkaObject
	if err := tx.Find(&objs).Error; err != nil {
		return err
	}
	visible := map[uuid.UUID]bool{}
	for _, o := range objs {
		visible[o.ID] = visibleObject(o) && o.ID != id
	}
	for _, r := range rows {
		ids := parseLevel(r.Content).ObjectIDs
		uses, others := false, false
		for _, oid := range ids {
			uses = uses || oid == id
			others = others || visible[oid]
		}
		if uses && !others {
			return ErrObjectLastInLevel
		}
	}
	return nil
}

// DeleteObject removes an object no level draft or version references.
func (s *AngkaAdminService) DeleteObject(adminID, id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		o, err := lockObject(tx, id)
		if err != nil {
			return err
		}
		refs, err := levelRefs(tx)
		if err != nil {
			return err
		}
		if len(refs[id]) > 0 {
			return ErrObjectInUse
		}
		var numbers int64
		if err := tx.Model(&models.AngkaNumber{}).
			Where("draft->>'object_id' = ? OR content->>'object_id' = ?", id.String(), id.String()).
			Count(&numbers).Error; err != nil {
			return err
		}
		if numbers > 0 {
			return ErrObjectInUse
		}
		if err := tx.Delete(o).Error; err != nil {
			return err
		}
		return s.audit.Record(tx, adminID, AuditDelete, AuditEntityAngkaObject, id.String(), parseObject(o.Draft), nil)
	})
}

// ─── Numbers ────────────────────────────────────────────────────────────────

// AdminNumberRow is one row of the Kenal Angka tab (from the draft).
type AdminNumberRow struct {
	Value          int        `json:"value"`
	Name           string     `json:"name"`
	AudioURL       string     `json:"audio_url"`
	ObjectID       *uuid.UUID `json:"object_id"`
	ObjectName     string     `json:"object_name"`
	ObjectImageURL string     `json:"object_image_url"`
	IsFree         bool       `json:"is_free"`
	adminStatus
}

// AdminNumberDraft is a number's editable state.
type AdminNumberDraft struct {
	AdminNumberRow
	Draft    models.AngkaNumberContent `json:"draft"`
	DraftRev int                       `json:"draft_rev"`
	Assets   map[uuid.UUID]AssetView   `json:"assets"`
}

func (s *AngkaAdminService) numberRows(tx *gorm.DB, where string, args ...interface{}) ([]AdminNumberDraft, error) {
	var nums []models.AngkaNumber
	q := tx.Order("value ASC")
	if where != "" {
		q = q.Where(where, args...)
	}
	if err := q.Find(&nums).Error; err != nil {
		return nil, err
	}
	drafts := make([]models.AngkaNumberContent, len(nums))
	var assetIDs, objectIDs []uuid.UUID
	var by []*uuid.UUID
	for i, n := range nums {
		drafts[i] = parseNumber(n.Draft)
		if id := drafts[i].AudioAssetID; id != nil {
			assetIDs = append(assetIDs, *id)
		}
		if id := drafts[i].ObjectID; id != nil {
			objectIDs = append(objectIDs, *id)
		}
		by = append(by, n.UpdatedBy)
	}
	assets, err := s.assets.Find(tx, assetIDs)
	if err != nil {
		return nil, err
	}
	objects, err := objectViews(tx, s.assets, objectIDs, true)
	if err != nil {
		return nil, err
	}
	objByID := map[uuid.UUID]AngkaObjectView{}
	for _, o := range objects {
		objByID[o.ID] = o
	}
	emails, err := s.adminEmails(by...)
	if err != nil {
		return nil, err
	}
	out := make([]AdminNumberDraft, len(nums))
	for i, n := range nums {
		d := drafts[i]
		row := AdminNumberRow{
			Value: n.Value, Name: d.Name, ObjectID: d.ObjectID, IsFree: n.IsFree,
			AudioURL: mediaURL(assets, d.AudioAssetID, d.AudioURL),
			adminStatus: adminStatus{
				Status:                n.Status,
				HasUnpublishedChanges: !sameJSON[models.AngkaNumberContent](n.Draft, n.Content),
				UpdatedBy:             emailOf(emails, n.UpdatedBy),
				UpdatedAt:             n.UpdatedAt,
			},
		}
		if d.ObjectID != nil {
			o := objByID[*d.ObjectID]
			row.ObjectName, row.ObjectImageURL = o.Name, o.ImageURL
		}
		mine := map[uuid.UUID]AssetView{}
		if id := d.AudioAssetID; id != nil {
			if a, ok := assets[*id]; ok {
				mine[*id] = a
			}
		}
		out[i] = AdminNumberDraft{AdminNumberRow: row, Draft: d, DraftRev: n.DraftRev, Assets: mine}
	}
	return out, nil
}

// ListNumbers returns numbers 1–20 with their drafts (the tab edits inline).
func (s *AngkaAdminService) ListNumbers() ([]AdminNumberDraft, error) {
	return s.numberRows(s.db, "")
}

func lockNumber(tx *gorm.DB, value int) (*models.AngkaNumber, error) {
	var n models.AngkaNumber
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("value = ?", value).First(&n).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAngkaNumberNotFound
	}
	return &n, err
}

func (s *AngkaAdminService) numberDraft(tx *gorm.DB, value int) (*AdminNumberDraft, error) {
	rows, err := s.numberRows(tx, "value = ?", value)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrAngkaNumberNotFound
	}
	return &rows[0], nil
}

// SaveNumberDraft stores a number's draft when rev matches.
func (s *AngkaAdminService) SaveNumberDraft(adminID uuid.UUID, value int, draft models.AngkaNumberContent, rev int) (*AdminNumberDraft, error) {
	var out *AdminNumberDraft
	err := s.db.Transaction(func(tx *gorm.DB) error {
		n, err := lockNumber(tx, value)
		if err != nil {
			return err
		}
		if n.DraftRev != rev {
			return ErrDraftConflict
		}
		normalizeNumber(&draft)
		if err := saveDraft(tx, n, adminID, draft); err != nil {
			return err
		}
		if err := s.audit.Record(tx, adminID, AuditDraftSave, AuditEntityAngkaNumber, fmt.Sprint(value), parseNumber(n.Draft), draft); err != nil {
			return err
		}
		out, err = s.numberDraft(tx, value)
		return err
	})
	return out, err
}

// PublishNumber validates the draft and makes it the published copy.
func (s *AngkaAdminService) PublishNumber(adminID uuid.UUID, value int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		n, err := lockNumber(tx, value)
		if err != nil {
			return err
		}
		c := parseNumber(n.Draft)
		var ids []uuid.UUID
		if c.AudioAssetID != nil {
			ids = append(ids, *c.AudioAssetID)
		}
		assets, err := s.assets.Find(tx, ids)
		if err != nil {
			return err
		}
		objects := map[uuid.UUID]models.AngkaObject{}
		if c.ObjectID != nil {
			var o models.AngkaObject
			if err := tx.Where("id = ?", *c.ObjectID).First(&o).Error; err == nil {
				objects[o.ID] = o
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if errs := validateNumber(c, assets, objects); len(errs) > 0 {
			return &ValidationFailedError{Fields: errs}
		}
		if err := publishCopy(tx, n, n.Status, n.Draft, adminID); err != nil {
			return err
		}
		return s.audit.Record(tx, adminID, AuditPublish, AuditEntityAngkaNumber, fmt.Sprint(value), parseNumber(n.Content), c)
	})
}

// SetNumberHidden hides or unhides a number in the Kenal Angka grid. Its
// published audio still counts aloud in levels.
func (s *AngkaAdminService) SetNumberHidden(adminID uuid.UUID, value int, hidden bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		n, err := lockNumber(tx, value)
		if err != nil {
			return err
		}
		return s.setStatus(tx, n, AuditEntityAngkaNumber, fmt.Sprint(value), n.Status, len(n.Content) > 0, hidden, adminID)
	})
}

// SetNumberFree sets whether a number opens without Akses Premium.
func (s *AngkaAdminService) SetNumberFree(adminID uuid.UUID, value int, free bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		n, err := lockNumber(tx, value)
		if err != nil {
			return err
		}
		if err := tx.Model(n).Updates(map[string]interface{}{
			"is_free": free, "updated_by": adminID, "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		action := AuditSetFree
		if !free {
			action = AuditUnsetFree
		}
		return s.audit.Record(tx, adminID, action, AuditEntityAngkaNumber, fmt.Sprint(value),
			map[string]bool{"is_free": n.IsFree}, map[string]bool{"is_free": free})
	})
}

// ─── Levels ─────────────────────────────────────────────────────────────────

// AdminLevelRow is one row of the levels table (from the draft).
type AdminLevelRow struct {
	ID                    uuid.UUID         `json:"id"`
	Name                  string            `json:"name"`
	SortOrder             int               `json:"sort_order"`
	IsFree                bool              `json:"is_free"`
	Version               *int              `json:"version"`
	Range                 models.AngkaRange `json:"range"`
	QuestionCount         int               `json:"question_count"`
	ObjectNames           []string          `json:"object_names"`
	Stars                 models.AngkaStars `json:"stars"`
	PrerequisiteID        *uuid.UUID        `json:"prerequisite_id"`
	PrerequisiteSortOrder *int              `json:"prerequisite_sort_order"`
	PrerequisiteName      string            `json:"prerequisite_name"`
	adminStatus
}

// AdminLevelDraft is a level's editable state.
type AdminLevelDraft struct {
	AdminLevelRow
	Draft    models.AngkaLevelContent `json:"draft"`
	DraftRev int                      `json:"draft_rev"`
}

type levelRow struct {
	models.AngkaLevel
	Version *int
	Content models.JSONB
}

func (s *AngkaAdminService) levelRows(tx *gorm.DB) ([]AdminLevelDraft, error) {
	var rows []levelRow
	if err := tx.Table("angka_levels AS l").
		Select("l.*, v.version, v.content").
		Joins("LEFT JOIN angka_level_versions v ON v.id = l.published_version_id").
		Order("l.sort_order ASC, l.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	var objs []models.AngkaObject
	if err := tx.Find(&objs).Error; err != nil {
		return nil, err
	}
	names := map[uuid.UUID]string{}
	for _, o := range objs {
		names[o.ID] = parseObject(o.Draft).Name
	}
	drafts := make([]models.AngkaLevelContent, len(rows))
	byID := map[uuid.UUID]int{}
	var by []*uuid.UUID
	for i, r := range rows {
		drafts[i] = parseLevel(r.Draft)
		byID[r.ID] = i
		by = append(by, r.UpdatedBy)
	}
	emails, err := s.adminEmails(by...)
	if err != nil {
		return nil, err
	}
	out := make([]AdminLevelDraft, len(rows))
	for i, r := range rows {
		d := drafts[i]
		on := make([]string, 0, len(d.ObjectIDs))
		for _, id := range d.ObjectIDs {
			if n, ok := names[id]; ok {
				on = append(on, n)
			}
		}
		row := AdminLevelRow{
			ID: r.ID, Name: d.Name, SortOrder: r.SortOrder, IsFree: r.IsFree, Version: r.Version,
			Range: d.Range, QuestionCount: d.QuestionCount, ObjectNames: on, Stars: d.Stars,
			PrerequisiteID: d.PrerequisiteID,
			adminStatus: adminStatus{
				Status:                r.Status,
				HasUnpublishedChanges: r.Version == nil || !sameJSON[models.AngkaLevelContent](r.Draft, r.Content),
				UpdatedBy:             emailOf(emails, r.UpdatedBy),
				UpdatedAt:             r.UpdatedAt,
			},
		}
		if p := d.PrerequisiteID; p != nil {
			if j, ok := byID[*p]; ok {
				order := rows[j].SortOrder
				row.PrerequisiteSortOrder = &order
				row.PrerequisiteName = drafts[j].Name
			}
		}
		out[i] = AdminLevelDraft{AdminLevelRow: row, Draft: d, DraftRev: r.DraftRev}
	}
	return out, nil
}

// ListLevels returns every level for the backoffice table.
func (s *AngkaAdminService) ListLevels() ([]AdminLevelRow, error) {
	rows, err := s.levelRows(s.db)
	if err != nil {
		return nil, err
	}
	out := make([]AdminLevelRow, len(rows))
	for i, r := range rows {
		out[i] = r.AdminLevelRow
	}
	return out, nil
}

// GetLevelDraft returns a level's draft.
func (s *AngkaAdminService) GetLevelDraft(id uuid.UUID) (*AdminLevelDraft, error) {
	return s.levelDraft(s.db, id)
}

func (s *AngkaAdminService) levelDraft(tx *gorm.DB, id uuid.UUID) (*AdminLevelDraft, error) {
	rows, err := s.levelRows(tx)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i], nil
		}
	}
	return nil, ErrAngkaLevelNotFound
}

// CreateLevel adds a draft level at the end, requiring the current last
// level.
func (s *AngkaAdminService) CreateLevel(adminID uuid.UUID) (*AdminLevelDraft, error) {
	var out *AdminLevelDraft
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var last models.AngkaLevel
		order := 1
		var prereq *uuid.UUID
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Order("sort_order DESC").First(&last).Error
		switch {
		case err == nil:
			order = last.SortOrder + 1
			id := last.ID
			prereq = &id
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}
		c := defaultLevelContent(fmt.Sprintf("Level %d", order), prereq)
		l := models.AngkaLevel{SortOrder: order, Draft: models.ToJSONB(c), UpdatedBy: &adminID}
		if err := tx.Create(&l).Error; err != nil {
			return err
		}
		if err := s.audit.Record(tx, adminID, AuditDraftSave, AuditEntityAngkaLevel, l.ID.String(), nil, c); err != nil {
			return err
		}
		out, err = s.levelDraft(tx, l.ID)
		return err
	})
	return out, err
}

func lockLevel(tx *gorm.DB, id uuid.UUID) (*models.AngkaLevel, error) {
	var l models.AngkaLevel
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&l).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAngkaLevelNotFound
	}
	return &l, err
}

// SaveLevelDraft stores a level's draft when rev matches.
func (s *AngkaAdminService) SaveLevelDraft(adminID, id uuid.UUID, draft models.AngkaLevelContent, rev int) (*AdminLevelDraft, error) {
	var out *AdminLevelDraft
	err := s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLevel(tx, id)
		if err != nil {
			return err
		}
		if l.DraftRev != rev {
			return ErrDraftConflict
		}
		normalizeLevel(&draft)
		if err := saveDraft(tx, l, adminID, draft); err != nil {
			return err
		}
		if err := s.audit.Record(tx, adminID, AuditDraftSave, AuditEntityAngkaLevel, id.String(), parseLevel(l.Draft), draft); err != nil {
			return err
		}
		out, err = s.levelDraft(tx, id)
		return err
	})
	return out, err
}

// levelChecks loads what level validation needs.
func (s *AngkaAdminService) levelChecks(tx *gorm.DB) (levelChecks, error) {
	chk := levelChecks{
		objects:       map[uuid.UUID]models.AngkaObject{},
		prerequisites: map[uuid.UUID]*uuid.UUID{},
		countAudio:    map[int]bool{},
	}
	var objs []models.AngkaObject
	if err := tx.Find(&objs).Error; err != nil {
		return chk, err
	}
	for _, o := range objs {
		chk.objects[o.ID] = o
	}
	var published []struct {
		ID      uuid.UUID
		Content models.JSONB
	}
	if err := tx.Table("angka_levels AS l").Select("l.id, v.content").
		Joins("JOIN angka_level_versions v ON v.id = l.published_version_id").
		Scan(&published).Error; err != nil {
		return chk, err
	}
	for _, p := range published {
		chk.prerequisites[p.ID] = parseLevel(p.Content).PrerequisiteID
	}
	audio, err := countAudio(tx, s.assets, angkaMaxNumber)
	if err != nil {
		return chk, err
	}
	for v := 1; v <= angkaMaxNumber; v++ {
		chk.countAudio[v] = audio[fmt.Sprint(v)] != ""
	}
	return chk, nil
}

// PublishLevel validates the draft and publishes it as the next version.
func (s *AngkaAdminService) PublishLevel(adminID, id uuid.UUID) (*models.AngkaLevelVersion, error) {
	var out *models.AngkaLevelVersion
	err := s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLevel(tx, id)
		if err != nil {
			return err
		}
		c := parseLevel(l.Draft)
		chk, err := s.levelChecks(tx)
		if err != nil {
			return err
		}
		if errs := validateLevel(id, c, chk); len(errs) > 0 {
			return &ValidationFailedError{Fields: errs}
		}
		v, err := s.newLevelVersion(tx, adminID, l, l.Draft)
		if err != nil {
			return err
		}
		out = v
		return s.audit.Record(tx, adminID, AuditPublish, AuditEntityAngkaLevel, id.String(), nil,
			map[string]interface{}{"version": v.Version})
	})
	return out, err
}

func (s *AngkaAdminService) newLevelVersion(tx *gorm.DB, adminID uuid.UUID, l *models.AngkaLevel, content models.JSONB) (*models.AngkaLevelVersion, error) {
	var next int
	if err := tx.Model(&models.AngkaLevelVersion{}).Where("level_id = ?", l.ID).
		Select("COALESCE(MAX(version), 0) + 1").Scan(&next).Error; err != nil {
		return nil, err
	}
	v := models.AngkaLevelVersion{LevelID: l.ID, Version: next, Content: content, PublishedBy: adminID}
	if err := tx.Create(&v).Error; err != nil {
		return nil, err
	}
	status := models.AngkaStatusPublished
	if l.Status == models.AngkaStatusHidden {
		status = models.AngkaStatusHidden
	}
	err := tx.Model(l).Updates(map[string]interface{}{
		"published_version_id": v.ID,
		"status":               status,
		"updated_by":           adminID,
		"updated_at":           time.Now(),
	}).Error
	return &v, err
}

// LevelVersions lists a level's versions, newest first.
func (s *AngkaAdminService) LevelVersions(id uuid.UUID) ([]VersionView, error) {
	var l models.AngkaLevel
	if err := s.db.Where("id = ?", id).First(&l).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAngkaLevelNotFound
		}
		return nil, err
	}
	out := []VersionView{}
	err := s.db.Table("angka_level_versions AS v").
		Select("v.id, v.version, a.email AS published_by, v.published_at").
		Joins("JOIN admin_users a ON a.id = v.published_by").
		Where("v.level_id = ?", id).Order("v.version DESC").Scan(&out).Error
	for i := range out {
		out[i].IsCurrent = l.PublishedVersionID != nil && out[i].ID == *l.PublishedVersionID
	}
	return out, err
}

// RollbackLevel republishes an old version's content as a new version and
// makes it the draft too.
func (s *AngkaAdminService) RollbackLevel(adminID, id uuid.UUID, version int) (*models.AngkaLevelVersion, error) {
	var out *models.AngkaLevelVersion
	err := s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLevel(tx, id)
		if err != nil {
			return err
		}
		var old models.AngkaLevelVersion
		if err := tx.Where("level_id = ? AND version = ?", id, version).First(&old).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrVersionNotFound
			}
			return err
		}
		v, err := s.newLevelVersion(tx, adminID, l, old.Content)
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
		return s.audit.Record(tx, adminID, AuditRollback, AuditEntityAngkaLevel, id.String(),
			map[string]interface{}{"from_version": version}, map[string]interface{}{"version": v.Version})
	})
	return out, err
}

// SetLevelHidden hides or unhides a level. Progress is never touched.
func (s *AngkaAdminService) SetLevelHidden(adminID, id uuid.UUID, hidden bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLevel(tx, id)
		if err != nil {
			return err
		}
		return s.setStatus(tx, l, AuditEntityAngkaLevel, id.String(), l.Status, l.PublishedVersionID != nil, hidden, adminID)
	})
}

// SetLevelFree makes id the only free level, or makes it premium again.
func (s *AngkaAdminService) SetLevelFree(adminID, id uuid.UUID, free bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLevel(tx, id)
		if err != nil {
			return err
		}
		if !free {
			if err := tx.Model(l).Updates(map[string]interface{}{
				"is_free": false, "updated_by": adminID, "updated_at": time.Now(),
			}).Error; err != nil {
				return err
			}
			return s.audit.Record(tx, adminID, AuditUnsetFree, AuditEntityAngkaLevel, id.String(),
				map[string]bool{"is_free": l.IsFree}, map[string]bool{"is_free": false})
		}
		var previous []uuid.UUID
		if err := tx.Model(&models.AngkaLevel{}).Where("is_free AND id <> ?", id).
			Pluck("id", &previous).Error; err != nil {
			return err
		}
		// Clear first: the partial unique index allows only one free row.
		if err := tx.Model(&models.AngkaLevel{}).Where("is_free AND id <> ?", id).
			Update("is_free", false).Error; err != nil {
			return err
		}
		if err := tx.Model(l).Updates(map[string]interface{}{
			"is_free": true, "updated_by": adminID, "updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		return s.audit.Record(tx, adminID, AuditSetFree, AuditEntityAngkaLevel, id.String(),
			map[string]interface{}{"free": previous}, map[string]interface{}{"free": id})
	})
}

// ReorderLevels sets sort_order 1..n in the order of ids, which must list
// every level exactly once.
func (s *AngkaAdminService) ReorderLevels(adminID uuid.UUID, ids []uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var all []models.AngkaLevel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Order("sort_order ASC").Find(&all).Error; err != nil {
			return err
		}
		if len(ids) != len(all) {
			return ErrInvalidOrder
		}
		known := map[uuid.UUID]bool{}
		before := make([]uuid.UUID, len(all))
		for i, l := range all {
			known[l.ID] = true
			before[i] = l.ID
		}
		seen := map[uuid.UUID]bool{}
		for _, id := range ids {
			if !known[id] || seen[id] {
				return ErrInvalidOrder
			}
			seen[id] = true
		}
		for i, id := range ids {
			if err := tx.Model(&models.AngkaLevel{}).Where("id = ?", id).
				Update("sort_order", i+1).Error; err != nil {
				return err
			}
		}
		return s.audit.Record(tx, adminID, AuditReorder, AuditEntityAngkaLevel, "order",
			map[string]interface{}{"order": before}, map[string]interface{}{"order": ids})
	})
}

// DeleteLevel removes a never-published level that no level requires.
func (s *AngkaAdminService) DeleteLevel(adminID, id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		l, err := lockLevel(tx, id)
		if err != nil {
			return err
		}
		var versions int64
		if err := tx.Model(&models.AngkaLevelVersion{}).Where("level_id = ?", id).Count(&versions).Error; err != nil {
			return err
		}
		if versions > 0 {
			return ErrLevelInUse
		}
		var others []models.AngkaLevel
		if err := tx.Where("id <> ?", id).Find(&others).Error; err != nil {
			return err
		}
		for _, o := range others {
			if p := parseLevel(o.Draft).PrerequisiteID; p != nil && *p == id {
				return ErrLevelInUse
			}
		}
		var required int64
		if err := tx.Model(&models.AngkaLevelVersion{}).
			Where("content->>'prerequisite_id' = ?", id.String()).Count(&required).Error; err != nil {
			return err
		}
		if required > 0 {
			return ErrLevelInUse
		}
		if err := tx.Delete(l).Error; err != nil {
			return err
		}
		return s.audit.Record(tx, adminID, AuditDelete, AuditEntityAngkaLevel, id.String(), parseLevel(l.Draft), nil)
	})
}

// PreviewQuestion is one generated question with its object resolved.
type PreviewQuestion struct {
	Count  int              `json:"count"`
	Size   int              `json:"size"`
	Layout string           `json:"layout"`
	Points []angkagen.Point `json:"points"`
	Object *AngkaObjectView `json:"object"`
}

// LevelPreview is what the backoffice's Pratinjau soal renders.
type LevelPreview struct {
	Seed       uint32            `json:"seed"`
	Questions  []PreviewQuestion `json:"questions"`
	CountAudio map[string]string `json:"count_audio"`
}

// PreviewLevel generates questions for an unsaved draft and seed, using the
// draft's objects that aren't hidden (draft media when never published).
func (s *AngkaAdminService) PreviewLevel(id uuid.UUID, draft models.AngkaLevelContent, seed uint32) (*LevelPreview, error) {
	var l models.AngkaLevel
	if err := s.db.Where("id = ?", id).First(&l).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAngkaLevelNotFound
		}
		return nil, err
	}
	normalizeLevel(&draft)
	if errs := validateLevelShape(draft); len(errs) > 0 {
		return nil, &ValidationFailedError{Fields: errs}
	}
	views, err := objectViews(s.db, s.assets, draft.ObjectIDs, true)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]AngkaObjectView{}
	var usable []uuid.UUID
	for _, v := range views {
		if !v.Hidden {
			byID[v.ID] = v
			usable = append(usable, v.ID)
		}
	}
	qs := generate(draft, usable, seed)
	out := &LevelPreview{Seed: seed, Questions: make([]PreviewQuestion, len(qs))}
	for i, q := range qs {
		pq := PreviewQuestion{Count: q.Count, Size: q.Size, Layout: q.Layout, Points: q.Points}
		if oid, err := uuid.Parse(q.ObjectID); err == nil {
			o := byID[oid]
			pq.Object = &o
		}
		out.Questions[i] = pq
	}
	out.CountAudio, err = countAudio(s.db, s.assets, draft.Range.Max)
	return out, err
}
