package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AngkaService is the app side of Belajar Angka: the manifest, published
// number and level content, play sessions and per-child progress. Access is
// the existing Akses Premium subscription; free numbers and the free level
// are open to everyone (design D1).
type AngkaService struct {
	db          *gorm.DB
	assets      *AssetService
	entitlement *EntitlementService
}

func NewAngkaService(db *gorm.DB, assets *AssetService, entitlement *EntitlementService) *AngkaService {
	return &AngkaService{db: db, assets: assets, entitlement: entitlement}
}

// ManifestNumber is one Kenal Angka tile.
type ManifestNumber struct {
	Value  int    `json:"value"`
	Name   string `json:"name"`
	IsFree bool   `json:"is_free"`
	Locked bool   `json:"locked"`
}

// ManifestLevel is one Hitung Benda level card. PrerequisiteID is set only
// when the prerequisite is itself visible; a hidden one counts as met.
type ManifestLevel struct {
	ID             uuid.UUID         `json:"id"`
	Name           string            `json:"name"`
	Range          models.AngkaRange `json:"range"`
	QuestionCount  int               `json:"question_count"`
	PrerequisiteID *uuid.UUID        `json:"prerequisite_id"`
	IsFree         bool              `json:"is_free"`
	Locked         bool              `json:"locked"`
	Version        int               `json:"version"`
}

// AngkaManifest is the published numbers and levels for one user.
type AngkaManifest struct {
	Premium bool             `json:"premium"`
	Numbers []ManifestNumber `json:"numbers"`
	Levels  []ManifestLevel  `json:"levels"`
}

type visibleLevel struct {
	ID        uuid.UUID
	IsFree    bool
	VersionID uuid.UUID
	Version   int
	Content   models.JSONB
}

// visibleLevels are published (not hidden) levels with their live version.
func visibleLevels(tx *gorm.DB, where string, args ...interface{}) ([]visibleLevel, error) {
	var rows []visibleLevel
	q := tx.Table("angka_levels AS l").
		Select("l.id, l.is_free, v.id AS version_id, v.version, v.content").
		Joins("JOIN angka_level_versions v ON v.id = l.published_version_id").
		Where("l.status = ?", models.AngkaStatusPublished).
		Order("l.sort_order ASC")
	if where != "" {
		q = q.Where(where, args...)
	}
	err := q.Scan(&rows).Error
	return rows, err
}

func visibleNumbers(tx *gorm.DB, where string, args ...interface{}) ([]models.AngkaNumber, error) {
	var rows []models.AngkaNumber
	q := tx.Where("status = ? AND content IS NOT NULL", models.AngkaStatusPublished).Order("value ASC")
	if where != "" {
		q = q.Where(where, args...)
	}
	err := q.Find(&rows).Error
	return rows, err
}

// Manifest returns the numbers and levels and an ETag over the whole body,
// which includes the caller's premium status.
func (s *AngkaService) Manifest(userID uuid.UUID) (*AngkaManifest, string, error) {
	premium, err := s.entitlement.HasActiveSubscription(userID)
	if err != nil {
		return nil, "", err
	}
	nums, err := visibleNumbers(s.db, "")
	if err != nil {
		return nil, "", err
	}
	levels, err := visibleLevels(s.db, "")
	if err != nil {
		return nil, "", err
	}
	m := &AngkaManifest{Premium: premium, Numbers: make([]ManifestNumber, len(nums)), Levels: make([]ManifestLevel, len(levels))}
	for i, n := range nums {
		m.Numbers[i] = ManifestNumber{
			Value: n.Value, Name: parseNumber(n.Content).Name,
			IsFree: n.IsFree, Locked: !n.IsFree && !premium,
		}
	}
	visible := map[uuid.UUID]bool{}
	for _, l := range levels {
		visible[l.ID] = true
	}
	for i, l := range levels {
		c := parseLevel(l.Content)
		prereq := c.PrerequisiteID
		if prereq != nil && !visible[*prereq] {
			prereq = nil
		}
		m.Levels[i] = ManifestLevel{
			ID: l.ID, Name: c.Name, Range: c.Range, QuestionCount: c.QuestionCount,
			PrerequisiteID: prereq, IsFree: l.IsFree, Locked: !l.IsFree && !premium, Version: l.Version,
		}
	}
	body, err := json.Marshal(m)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(body)
	return m, `"` + hex.EncodeToString(sum[:16]) + `"`, nil
}

func (s *AngkaService) canAccess(userID uuid.UUID, isFree bool) error {
	if isFree {
		return nil
	}
	premium, err := s.entitlement.HasActiveSubscription(userID)
	if err != nil {
		return err
	}
	if !premium {
		return ErrPremiumRequired
	}
	return nil
}

// AngkaNumberView is a Kenal Angka card.
type AngkaNumberView struct {
	Value      int               `json:"value"`
	Name       string            `json:"name"`
	AudioURL   string            `json:"audio_url"`
	Object     *AngkaObjectView  `json:"object"`
	CountAudio map[string]string `json:"count_audio"`
}

// Number returns a visible number's card, with the audio to count 1..value.
func (s *AngkaService) Number(userID uuid.UUID, value int) (*AngkaNumberView, error) {
	nums, err := visibleNumbers(s.db, "value = ?", value)
	if err != nil {
		return nil, err
	}
	if len(nums) == 0 {
		return nil, ErrAngkaNumberNotFound
	}
	n := nums[0]
	if err := s.canAccess(userID, n.IsFree); err != nil {
		return nil, err
	}
	c := parseNumber(n.Content)
	var ids []uuid.UUID
	if c.AudioAssetID != nil {
		ids = append(ids, *c.AudioAssetID)
	}
	assets, err := s.assets.Find(s.db, ids)
	if err != nil {
		return nil, err
	}
	v := &AngkaNumberView{Value: n.Value, Name: c.Name, AudioURL: mediaURL(assets, c.AudioAssetID, c.AudioURL)}
	if c.ObjectID != nil {
		objs, err := objectViews(s.db, s.assets, []uuid.UUID{*c.ObjectID}, false)
		if err != nil {
			return nil, err
		}
		if len(objs) == 1 {
			v.Object = &objs[0]
		}
	}
	v.CountAudio, err = countAudio(s.db, s.assets, n.Value)
	return v, err
}

// AngkaLevelView is a level's published content with its media resolved.
type AngkaLevelView struct {
	ID      uuid.UUID `json:"id"`
	Version int       `json:"version"`
	models.AngkaLevelContent
	Objects    []AngkaObjectView `json:"objects"`
	CountAudio map[string]string `json:"count_audio"`
}

func (s *AngkaService) levelView(tx *gorm.DB, id uuid.UUID, version int, c models.AngkaLevelContent, objectIDs []uuid.UUID) (*AngkaLevelView, error) {
	objs, err := objectViews(tx, s.assets, objectIDs, false)
	if err != nil {
		return nil, err
	}
	audio, err := countAudio(tx, s.assets, c.Range.Max)
	if err != nil {
		return nil, err
	}
	return &AngkaLevelView{ID: id, Version: version, AngkaLevelContent: c, Objects: objs, CountAudio: audio}, nil
}

// Level returns a visible level's published content. Objects include
// hidden ones, so sessions started before an object was hidden still play.
func (s *AngkaService) Level(userID, id uuid.UUID) (*AngkaLevelView, error) {
	rows, err := visibleLevels(s.db, "l.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrAngkaLevelNotFound
	}
	r := rows[0]
	if err := s.canAccess(userID, r.IsFree); err != nil {
		return nil, err
	}
	c := parseLevel(r.Content)
	return s.levelView(s.db, r.ID, r.Version, c, c.ObjectIDs)
}

func (s *AngkaService) ownedChild(userID, childID uuid.UUID) error {
	var n int64
	if err := s.db.Model(&models.Children{}).
		Where("id = ? AND parent_id = ? AND is_deleted = false", childID, userID).
		Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return ErrChildNotFound
	}
	return nil
}

// ─── Progress ───────────────────────────────────────────────────────────────

// AngkaCurrentSession is a level's in-progress session, for "Lanjut".
type AngkaCurrentSession struct {
	ID            uuid.UUID            `json:"id"`
	LevelVersion  int                  `json:"level_version"`
	Seed          uint32               `json:"seed"`
	ObjectIDs     []uuid.UUID          `json:"object_ids"`
	Answers       []models.AngkaAnswer `json:"answers"`
	QuestionCount int                  `json:"question_count"`
	Answered      int                  `json:"answered"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// AngkaLevelProgressView is one level's progress.
type AngkaLevelProgressView struct {
	LevelID        uuid.UUID            `json:"level_id"`
	BestStars      int                  `json:"best_stars"`
	Completed      bool                 `json:"completed"`
	CurrentSession *AngkaCurrentSession `json:"current_session"`
}

// Progress returns the child's progress on every level they've played.
func (s *AngkaService) Progress(userID, childID uuid.UUID) ([]AngkaLevelProgressView, error) {
	if err := s.ownedChild(userID, childID); err != nil {
		return nil, err
	}
	if err := abandonStale(s.db, childID); err != nil {
		return nil, err
	}
	var rows []models.AngkaLevelProgress
	if err := s.db.Where("child_id = ?", childID).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AngkaLevelProgressView, 0, len(rows))
	for _, r := range rows {
		v := AngkaLevelProgressView{LevelID: r.LevelID, BestStars: r.BestStars, Completed: r.Completed}
		if r.CurrentSessionID != nil {
			sess, content, version, err := loadSession(s.db, childID, *r.CurrentSessionID, false)
			if err != nil && !errors.Is(err, ErrAngkaSessionNotFound) {
				return nil, err
			}
			if err == nil && sess.Status == models.AngkaSessionInProgress {
				answers := sessionAnswers(sess)
				objectIDs := sessionObjects(sess)
				qs := generate(content, objectIDs, uint32(sess.Seed))
				v.CurrentSession = &AngkaCurrentSession{
					ID: sess.ID, LevelVersion: version, Seed: uint32(sess.Seed),
					ObjectIDs: objectIDs, Answers: answers,
					QuestionCount: len(qs), Answered: finishedCount(qs, answers),
					UpdatedAt: sess.UpdatedAt,
				}
			}
		}
		out = append(out, v)
	}
	return out, nil
}
