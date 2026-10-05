package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrPremiumRequired means the letter needs an active Akses Premium
// subscription (402 PREMIUM_REQUIRED).
var ErrPremiumRequired = errors.New("premium subscription required")

// HurufService is the app side of Belajar Huruf: the manifest, published
// letter content and per-child progress. Access is the existing Akses
// Premium subscription; one letter (is_free) is open to everyone.
type HurufService struct {
	db          *gorm.DB
	assets      *AssetService
	entitlement *EntitlementService
	thresholds  TracingThresholds
}

func NewHurufService(db *gorm.DB, assets *AssetService, entitlement *EntitlementService) *HurufService {
	return &HurufService{db: db, assets: assets, entitlement: entitlement, thresholds: TracingThresholdsFromEnv()}
}

// TracingThresholds tune the app's on-device tracing evaluator without an
// app release (design D6).
type TracingThresholds struct {
	Radius         float64 `json:"radius"`
	Coverage       float64 `json:"coverage"`
	Accuracy       float64 `json:"accuracy"`
	Direction      float64 `json:"direction"`
	StartTolerance float64 `json:"start_tolerance"`
	MaxFailures    int     `json:"max_failures"`
}

// TracingThresholdsFromEnv reads HURUF_TRACING_* with the RFC defaults.
func TracingThresholdsFromEnv() TracingThresholds {
	f := func(key string, def float64) float64 {
		if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil && v > 0 {
			return v
		}
		return def
	}
	return TracingThresholds{
		Radius:         f("HURUF_TRACING_RADIUS", 22),
		Coverage:       f("HURUF_TRACING_COVERAGE", 0.80),
		Accuracy:       f("HURUF_TRACING_ACCURACY", 0.85),
		Direction:      f("HURUF_TRACING_DIRECTION", 0.70),
		StartTolerance: f("HURUF_TRACING_START_TOLERANCE", 30),
		MaxFailures:    int(f("HURUF_TRACING_MAX_FAILURES", 3)),
	}
}

// ManifestLetter is one tile of the app's letter grid.
type ManifestLetter struct {
	ID       uuid.UUID `json:"id"`
	Upper    string    `json:"upper"`
	Lower    string    `json:"lower"`
	Word     string    `json:"word"`
	ImageURL string    `json:"image_url"`
	IsFree   bool      `json:"is_free"`
	Locked   bool      `json:"locked"`
	Version  int       `json:"version"`
}

// Manifest is the published letter list for one user.
type Manifest struct {
	Premium           bool              `json:"premium"`
	Letters           []ManifestLetter  `json:"letters"`
	TracingThresholds TracingThresholds `json:"tracing_thresholds"`
}

type publishedRow struct {
	ID      uuid.UUID
	Upper   string
	IsFree  bool
	Version int
	Content models.JSONB
}

// visibleLetters are published (not hidden) letters with their live version.
func (s *HurufService) visibleLetters(where string, args ...interface{}) ([]publishedRow, error) {
	var rows []publishedRow
	q := s.db.Table("letters AS l").
		Select("l.id, l.upper, l.is_free, v.version, v.content").
		Joins("JOIN letter_versions v ON v.id = l.published_version_id").
		Where("l.status = ?", models.LetterStatusPublished).
		Order("l.sort_order ASC")
	if where != "" {
		q = q.Where(where, args...)
	}
	err := q.Scan(&rows).Error
	return rows, err
}

// Manifest returns the letter grid and its ETag. The ETag is a hash of the
// whole body, so it changes with any published content, order, visibility,
// free letter, threshold — or the caller's premium status.
func (s *HurufService) Manifest(userID uuid.UUID) (*Manifest, string, error) {
	premium, err := s.entitlement.HasActiveSubscription(userID)
	if err != nil {
		return nil, "", err
	}
	rows, err := s.visibleLetters("")
	if err != nil {
		return nil, "", err
	}
	contents := make([]models.LetterContent, len(rows))
	var ids []uuid.UUID
	for i, r := range rows {
		contents[i], _ = parseContent(r.Content)
		if id := contents[i].Kenali.ImageAssetID; id != nil {
			ids = append(ids, *id)
		}
	}
	assets, err := s.assets.Find(s.db, ids)
	if err != nil {
		return nil, "", err
	}
	m := &Manifest{Premium: premium, Letters: make([]ManifestLetter, len(rows)), TracingThresholds: s.thresholds}
	for i, r := range rows {
		c := contents[i]
		m.Letters[i] = ManifestLetter{
			ID: r.ID, Upper: r.Upper, Lower: strings.ToLower(r.Upper),
			Word: c.Kenali.Word, ImageURL: mediaURL(assets, c.Kenali.ImageAssetID, c.Kenali.ImageURL),
			IsFree: r.IsFree, Locked: !r.IsFree && !premium, Version: r.Version,
		}
	}
	body, err := json.Marshal(m)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(body)
	return m, `"` + hex.EncodeToString(sum[:16]) + `"`, nil
}

// canAccess reports whether userID may open letter r.
func (s *HurufService) canAccess(userID uuid.UUID, isFree bool) error {
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

// Letter returns a visible letter's published content.
func (s *HurufService) Letter(userID, id uuid.UUID) (*LetterView, error) {
	rows, err := s.visibleLetters("l.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrLetterNotFound
	}
	r := rows[0]
	if err := s.canAccess(userID, r.IsFree); err != nil {
		return nil, err
	}
	c, err := parseContent(r.Content)
	if err != nil {
		return nil, err
	}
	return buildLetterView(s.db, s.assets, r.ID, r.Version, c)
}

func (s *HurufService) ownedChild(userID, childID uuid.UUID) error {
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

// Progress returns all of a child's letter progress rows.
func (s *HurufService) Progress(userID, childID uuid.UUID) ([]models.LetterProgress, error) {
	if err := s.ownedChild(userID, childID); err != nil {
		return nil, err
	}
	rows := []models.LetterProgress{}
	err := s.db.Where("child_id = ?", childID).Order("updated_at DESC").Find(&rows).Error
	return rows, err
}

// ProgressUpdate is the PUT body. Flags only ever turn on (OR-merged), so a
// stale or retried write can never clear a completion.
type ProgressUpdate struct {
	KenaliDone   bool     `json:"kenali_done"`
	DengarDone   bool     `json:"dengar_done"`
	TebalkanDone bool     `json:"tebalkan_done"`
	Score        *float64 `json:"score"`
	Attempt      bool     `json:"attempt"`
}

// ErrInvalidScore rejects a score outside 0..1.
var ErrInvalidScore = errors.New("score must be between 0 and 1")

// UpdateProgress merges u into the child's progress on a visible letter.
func (s *HurufService) UpdateProgress(userID, childID, letterID uuid.UUID, u ProgressUpdate) (*models.LetterProgress, error) {
	if u.Score != nil && (*u.Score < 0 || *u.Score > 1) {
		return nil, ErrInvalidScore
	}
	if err := s.ownedChild(userID, childID); err != nil {
		return nil, err
	}
	rows, err := s.visibleLetters("l.id = ?", letterID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrLetterNotFound
	}
	if err := s.canAccess(userID, rows[0].IsFree); err != nil {
		return nil, err
	}
	attempts := 0
	if u.Attempt {
		attempts = 1
	}
	var score interface{}
	if u.Score != nil {
		score = *u.Score
	}
	var out models.LetterProgress
	err = s.db.Raw(`
		INSERT INTO letter_progress
			(child_id, letter_id, kenali_done, dengar_done, tebalkan_done, best_score, attempts, updated_at)
		VALUES (?, ?, ?, ?, ?, ROUND(?::numeric, 2), ?, ?)
		ON CONFLICT (child_id, letter_id) DO UPDATE SET
			kenali_done   = letter_progress.kenali_done   OR EXCLUDED.kenali_done,
			dengar_done   = letter_progress.dengar_done   OR EXCLUDED.dengar_done,
			tebalkan_done = letter_progress.tebalkan_done OR EXCLUDED.tebalkan_done,
			best_score    = GREATEST(letter_progress.best_score, EXCLUDED.best_score),
			attempts      = letter_progress.attempts + EXCLUDED.attempts,
			updated_at    = EXCLUDED.updated_at
		RETURNING *`,
		childID, letterID, u.KenaliDone, u.DengarDone, u.TebalkanDone, score, attempts, time.Now(),
	).Scan(&out).Error
	return &out, err
}
