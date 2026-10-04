package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"arunika_backend/growth"
	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrChildNotFound covers both "no such child" and "it belongs to someone
// else". Callers must answer both identically, or the endpoint becomes an
// oracle for which child ids exist. ErrMeasurementNotFound is the same for a
// measurement id.
var (
	ErrChildNotFound       = errors.New("child not found")
	ErrMeasurementNotFound = errors.New("measurement not found")
	ErrClientIDConflict    = errors.New("client_id already used for another child")
)

// Growth validation error codes, returned to the app as 422.
const (
	GrowthProfileIncomplete   = "PROFILE_INCOMPLETE"
	GrowthDateOutOfRange      = "DATE_OUT_OF_RANGE"
	GrowthValueRequired       = "VALUE_REQUIRED"
	GrowthInvalidValue        = "INVALID_VALUE"
	GrowthInvalidPosition     = "INVALID_POSITION"
	GrowthOutlierNeedsConfirm = "OUTLIER_NEEDS_CONFIRM"
)

// GrowthValidationError is a request the server refuses to store. Indicators
// lists the implausible ones ("hfa", "wfa") for OUTLIER_NEEDS_CONFIRM.
type GrowthValidationError struct {
	Code       string
	Indicators []string
}

func (e *GrowthValidationError) Error() string { return e.Code }

func invalid(code string) error { return &GrowthValidationError{Code: code} }

// growthTZ is the parents' calendar: "today" for date validation is in WIB.
// Indonesia has no DST, and a fixed zone needs no tzdata in the image.
var growthTZ = time.FixedZone("WIB", 7*60*60)

const growthDateLayout = "2006-01-02"

// growthPurgeAfter is how long a soft-deleted measurement can still be
// restored before the purge job removes it.
const growthPurgeAfter = 30 * 24 * time.Hour

type GrowthService struct {
	db  *gorm.DB
	now func() time.Time
}

func NewGrowthService(db *gorm.DB) *GrowthService {
	return &GrowthService{db: db, now: time.Now}
}

// ownedChild loads one of userID's (non-deleted) children.
func (s *GrowthService) ownedChild(tx *gorm.DB, userID, childID uuid.UUID) (*models.Children, error) {
	var child models.Children
	err := tx.Where("id = ? AND parent_id = ? AND is_deleted = false", childID, userID).First(&child).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrChildNotFound
	}
	if err != nil {
		return nil, err
	}
	return &child, nil
}

func (s *GrowthService) measurement(tx *gorm.DB, childID, id uuid.UUID, includeDeleted bool) (*models.GrowthMeasurement, error) {
	q := tx.Where("id = ? AND child_id = ?", id, childID)
	if !includeDeleted {
		q = q.Where("deleted_at IS NULL")
	}
	var m models.GrowthMeasurement
	err := q.First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMeasurementNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// childProfile returns the child's sex and birth date, or ok=false when either
// is missing or unrecognized.
func childProfile(child *models.Children) (growth.Sex, time.Time, bool) {
	sex, ok := growth.NormalizeSex(child.Gender)
	if !ok || child.DateOfBirth.IsZero() {
		return "", time.Time{}, false
	}
	return sex, child.DateOfBirth, true
}

// ─── Requests ─────────────────────────────────────────────────────────────────

// OptionalFloat distinguishes a missing JSON field (Set=false) from an
// explicit null (Set=true, Value=nil), so PATCH can clear a value.
type OptionalFloat struct {
	Set   bool
	Value *float64
}

func (o *OptionalFloat) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Value = nil
		return nil
	}
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

type CreateMeasurementRequest struct {
	ClientID       *uuid.UUID `json:"client_id"`
	MeasuredOn     string     `json:"measured_on" binding:"required"`
	HeightCm       *float64   `json:"height_cm"`
	WeightKg       *float64   `json:"weight_kg"`
	Position       string     `json:"position" binding:"required"`
	ConfirmOutlier bool       `json:"confirm_outlier"`
}

type UpdateMeasurementRequest struct {
	MeasuredOn     *string       `json:"measured_on"`
	HeightCm       OptionalFloat `json:"height_cm"`
	WeightKg       OptionalFloat `json:"weight_kg"`
	Position       *string       `json:"position"`
	ConfirmOutlier bool          `json:"confirm_outlier"`
}

// ─── Responses ────────────────────────────────────────────────────────────────

type IndicatorView struct {
	Z        *float64 `json:"z"`
	Category string   `json:"category"`
}

type MeasurementView struct {
	ID         uuid.UUID      `json:"id"`
	ClientID   *uuid.UUID     `json:"client_id"`
	MeasuredOn string         `json:"measured_on"`
	HeightCm   *float64       `json:"height_cm"`
	WeightKg   *float64       `json:"weight_kg"`
	Position   string         `json:"position"`
	AgeDays    int            `json:"age_days"`
	AgeLabel   string         `json:"age_label"`
	HFA        *IndicatorView `json:"hfa"`
	WFA        *IndicatorView `json:"wfa"`
	Flagged    bool           `json:"flagged"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type LatestValue struct {
	Value             float64  `json:"value"`
	Category          *string  `json:"category"`
	MeasuredOn        string   `json:"measured_on"`
	DeltaFromPrevious *float64 `json:"delta_from_previous,omitempty"`
}

type GrowthChild struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Sex          string    `json:"sex"`
	BirthDate    string    `json:"birth_date"`
	AgeLabel     string    `json:"age_label"`
	Over60Months bool      `json:"over_60_months"`
}

type GrowthSummary struct {
	ProfileComplete bool              `json:"profile_complete"`
	Child           GrowthChild       `json:"child"`
	LatestHeight    *LatestValue      `json:"latest_height"`
	LatestWeight    *LatestValue      `json:"latest_weight"`
	Measurements    []MeasurementView `json:"measurements"`
}

func toView(m models.GrowthMeasurement) MeasurementView {
	v := MeasurementView{
		ID: m.ID, ClientID: m.ClientID, MeasuredOn: m.MeasuredOn.Format(growthDateLayout),
		HeightCm: m.HeightCm, WeightKg: m.WeightKg, Position: m.Position,
		AgeDays: m.AgeDays, AgeLabel: growth.AgeLabel(max(m.AgeDays, 0)), Flagged: m.Flagged,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.HfaCategory != nil {
		v.HFA = &IndicatorView{Z: m.HfaZ, Category: *m.HfaCategory}
	}
	if m.WfaCategory != nil {
		v.WFA = &IndicatorView{Z: m.WfaZ, Category: *m.WfaCategory}
	}
	return v
}

// ─── Classification ───────────────────────────────────────────────────────────

// classifyInto recomputes the derived columns of m. It returns the implausible
// indicators. With an incomplete profile, or a measurement dated before birth,
// the derived columns are cleared.
func classifyInto(m *models.GrowthMeasurement, child *models.Children) []string {
	m.HfaZ, m.WfaZ, m.HfaCategory, m.WfaCategory, m.Flagged = nil, nil, nil, nil, false
	m.WHOVersion = growth.WHOVersion
	sex, birth, ok := childProfile(child)
	if !ok {
		return nil
	}
	m.AgeDays = growth.AgeDays(birth, m.MeasuredOn)
	res, err := growth.Classify(growth.Input{
		Sex: sex, BirthDate: birth, MeasuredOn: m.MeasuredOn,
		HeightCm: m.HeightCm, WeightKg: m.WeightKg, Position: growth.Position(m.Position),
	})
	if err != nil {
		return nil
	}
	var flagged []string
	if res.HFA != nil {
		m.HfaZ, m.HfaCategory = res.HFA.Z, &res.HFA.Category
		if res.HFA.Flagged {
			flagged = append(flagged, "hfa")
		}
	}
	if res.WFA != nil {
		m.WfaZ, m.WfaCategory = res.WFA.Z, &res.WFA.Category
		if res.WFA.Flagged {
			flagged = append(flagged, "wfa")
		}
	}
	m.Flagged = len(flagged) > 0
	return flagged
}

// validate checks a fully-merged measurement before it is stored, and
// classifies it.
func (s *GrowthService) validate(m *models.GrowthMeasurement, child *models.Children, confirmOutlier bool) error {
	_, birth, ok := childProfile(child)
	if !ok {
		return invalid(GrowthProfileIncomplete)
	}
	if m.HeightCm == nil && m.WeightKg == nil {
		return invalid(GrowthValueRequired)
	}
	for _, v := range []*float64{m.HeightCm, m.WeightKg} {
		if v != nil && (*v <= 0 || *v >= 1000) {
			return invalid(GrowthInvalidValue)
		}
	}
	if m.Position != string(growth.Standing) && m.Position != string(growth.Recumbent) {
		return invalid(GrowthInvalidPosition)
	}
	today := s.now().In(growthTZ)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if m.MeasuredOn.After(todayDate) || growth.AgeDays(birth, m.MeasuredOn) < 0 {
		return invalid(GrowthDateOutOfRange)
	}
	if flagged := classifyInto(m, child); len(flagged) > 0 && !confirmOutlier {
		return &GrowthValidationError{Code: GrowthOutlierNeedsConfirm, Indicators: flagged}
	}
	return nil
}

func parseGrowthDate(s string) (time.Time, error) {
	d, err := time.Parse(growthDateLayout, s)
	if err != nil {
		return time.Time{}, invalid(GrowthDateOutOfRange)
	}
	return d, nil
}

// roundTenth keeps inputs at the column's precision (numeric(4,1)).
func roundTenth(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := math.Round(*v*10) / 10
	return &r
}

// ─── Operations ───────────────────────────────────────────────────────────────

// Summary returns the child's growth summary and all measurements, newest first.
func (s *GrowthService) Summary(userID, childID uuid.UUID) (*GrowthSummary, error) {
	child, err := s.ownedChild(s.db, userID, childID)
	if err != nil {
		return nil, err
	}
	var rows []models.GrowthMeasurement
	if err := s.db.Where("child_id = ? AND deleted_at IS NULL", childID).
		Order("measured_on DESC, created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}

	sum := &GrowthSummary{
		Child:        GrowthChild{ID: child.ID, Name: child.Name},
		Measurements: make([]MeasurementView, 0, len(rows)),
	}
	if sex, birth, ok := childProfile(child); ok {
		age := growth.AgeDays(birth, s.now().In(growthTZ))
		sum.ProfileComplete = true
		sum.Child.Sex = string(sex)
		sum.Child.BirthDate = birth.Format(growthDateLayout)
		sum.Child.AgeLabel = growth.AgeLabel(max(age, 0))
		sum.Child.Over60Months = growth.AgeMonths(age) > 60
	}
	for _, r := range rows {
		sum.Measurements = append(sum.Measurements, toView(r))
	}
	sum.LatestHeight = latest(rows, func(m models.GrowthMeasurement) (*float64, *string) { return m.HeightCm, m.HfaCategory })
	sum.LatestWeight = latest(rows, func(m models.GrowthMeasurement) (*float64, *string) { return m.WeightKg, m.WfaCategory })
	return sum, nil
}

// latest picks the newest row with a value, and its change from the next
// older row with a value. rows must be newest first.
func latest(rows []models.GrowthMeasurement, pick func(models.GrowthMeasurement) (*float64, *string)) *LatestValue {
	var out *LatestValue
	for _, r := range rows {
		v, cat := pick(r)
		if v == nil {
			continue
		}
		if out == nil {
			out = &LatestValue{Value: *v, Category: cat, MeasuredOn: r.MeasuredOn.Format(growthDateLayout)}
			continue
		}
		d := math.Round((out.Value-*v)*10) / 10
		out.DeltaFromPrevious = &d
		break
	}
	return out
}

// Create stores a new measurement. created is false when client_id matched an
// existing row of the same child, which is returned unchanged.
func (s *GrowthService) Create(userID, childID uuid.UUID, req CreateMeasurementRequest) (view *MeasurementView, created bool, err error) {
	child, err := s.ownedChild(s.db, userID, childID)
	if err != nil {
		return nil, false, err
	}
	if req.ClientID != nil {
		var existing models.GrowthMeasurement
		err := s.db.Where("client_id = ?", *req.ClientID).First(&existing).Error
		if err == nil {
			if existing.ChildID != childID {
				return nil, false, ErrClientIDConflict
			}
			v := toView(existing)
			return &v, false, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, err
		}
	}
	on, err := parseGrowthDate(req.MeasuredOn)
	if err != nil {
		return nil, false, err
	}
	m := models.GrowthMeasurement{
		ChildID: childID, ClientID: req.ClientID, MeasuredOn: on,
		HeightCm: roundTenth(req.HeightCm), WeightKg: roundTenth(req.WeightKg), Position: req.Position,
	}
	if err := s.validate(&m, child, req.ConfirmOutlier); err != nil {
		return nil, false, err
	}
	if err := s.db.Create(&m).Error; err != nil {
		return nil, false, err
	}
	v := toView(m)
	return &v, true, nil
}

// Update applies the provided fields and recomputes the classification.
func (s *GrowthService) Update(userID, childID, id uuid.UUID, req UpdateMeasurementRequest) (*MeasurementView, error) {
	child, err := s.ownedChild(s.db, userID, childID)
	if err != nil {
		return nil, err
	}
	m, err := s.measurement(s.db, childID, id, false)
	if err != nil {
		return nil, err
	}
	if req.MeasuredOn != nil {
		if m.MeasuredOn, err = parseGrowthDate(*req.MeasuredOn); err != nil {
			return nil, err
		}
	}
	if req.HeightCm.Set {
		m.HeightCm = roundTenth(req.HeightCm.Value)
	}
	if req.WeightKg.Set {
		m.WeightKg = roundTenth(req.WeightKg.Value)
	}
	if req.Position != nil {
		m.Position = *req.Position
	}
	if err := s.validate(m, child, req.ConfirmOutlier); err != nil {
		return nil, err
	}
	m.UpdatedAt = s.now()
	// Select("*") so cleared (nil) values are written too.
	if err := s.db.Model(m).Select("*").Omit("id", "created_at", "client_id").Updates(m).Error; err != nil {
		return nil, err
	}
	v := toView(*m)
	return &v, nil
}

// Delete soft-deletes a measurement so it can still be restored.
func (s *GrowthService) Delete(userID, childID, id uuid.UUID) error {
	if _, err := s.ownedChild(s.db, userID, childID); err != nil {
		return err
	}
	m, err := s.measurement(s.db, childID, id, false)
	if err != nil {
		return err
	}
	return s.db.Model(m).Update("deleted_at", s.now()).Error
}

// Restore undoes a soft delete that has not been purged yet.
func (s *GrowthService) Restore(userID, childID, id uuid.UUID) (*MeasurementView, error) {
	if _, err := s.ownedChild(s.db, userID, childID); err != nil {
		return nil, err
	}
	m, err := s.measurement(s.db, childID, id, true)
	if err != nil {
		return nil, err
	}
	if m.DeletedAt != nil {
		if err := s.db.Model(m).Update("deleted_at", nil).Error; err != nil {
			return nil, err
		}
		m.DeletedAt = nil
	}
	v := toView(*m)
	return &v, nil
}

// PurgeDeleted hard-deletes measurements soft-deleted more than 30 days ago.
func (s *GrowthService) PurgeDeleted() (int64, error) {
	res := s.db.Where("deleted_at IS NOT NULL AND deleted_at < ?", s.now().Add(-growthPurgeAfter)).
		Delete(&models.GrowthMeasurement{})
	return res.RowsAffected, res.Error
}

// RecomputeChildGrowth reclassifies every measurement of a child (including
// soft-deleted ones, so a restore is consistent) after its birth date or sex
// changed. Run it in the transaction that changed the child.
func RecomputeChildGrowth(tx *gorm.DB, child *models.Children) error {
	var rows []models.GrowthMeasurement
	if err := tx.Where("child_id = ?", child.ID).Find(&rows).Error; err != nil {
		return fmt.Errorf("load growth measurements: %w", err)
	}
	for i := range rows {
		m := &rows[i]
		classifyInto(m, child)
		if err := tx.Model(m).Select("age_days", "hfa_z", "wfa_z", "hfa_category", "wfa_category", "flagged", "who_version").
			Updates(m).Error; err != nil {
			return fmt.Errorf("recompute growth measurement %s: %w", m.ID, err)
		}
	}
	return nil
}
