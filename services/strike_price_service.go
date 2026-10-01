package services

import (
	"arunika_backend/models"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// MaxPromoDays caps how long a strike-price promo may run. The cap is what
// keeps a strike price an honest, time-limited promo rather than a
// permanent, never-charged "normal price".
const MaxPromoDays = 90

// Status values reported for a rule or override in the backoffice.
const (
	StrikeStatusOff       = "OFF"
	StrikeStatusScheduled = "SCHEDULED"
	StrikeStatusActive    = "ACTIVE"
	StrikeStatusEnded     = "ENDED"
)

// ValidationError marks a bad admin input so handlers can answer 400
// instead of 500.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

func validationErrorf(format string, args ...interface{}) error {
	return &ValidationError{msg: fmt.Sprintf(format, args...)}
}

// IsValidationError reports whether err (or anything it wraps) is a
// ValidationError.
func IsValidationError(err error) bool {
	var v *ValidationError
	return errors.As(err, &v)
}

// StrikeRules maps scope -> global rule. A nil StrikeRules is valid and
// simply has no global rules (items can still use their own override).
type StrikeRules map[string]models.StrikePriceRule

type StrikePriceService struct {
	db *gorm.DB
}

func NewStrikePriceService(db *gorm.DB) *StrikePriceService {
	return &StrikePriceService{db: db}
}

// LoadRules reads the three global rules. Callers load them once per
// request and resolve every item against the result in memory. Safe to call
// on a nil receiver (returns no rules), so services wired without strike
// pricing keep working unchanged.
func (s *StrikePriceService) LoadRules() (StrikeRules, error) {
	if s == nil {
		return nil, nil
	}
	rules, err := models.FindAllStrikePriceRules(s.db)
	if err != nil {
		return nil, err
	}
	out := make(StrikeRules, len(rules))
	for _, r := range rules {
		out[r.Scope] = r
	}
	return out, nil
}

// ComputeStrikePrice returns the display-only strike price and discount
// badge for a real price under one rule, or nils when there is none.
//   - PERCENT p: strike = price ÷ (1 − p/100), rounded up to the nearest
//     Rp 1.000 — so "-p%" reads as a discount off the strike price.
//   - FIXED a:   strike = price + a.
//
// discountPct is derived from the final rounded numbers so the badge always
// matches what is on screen.
func ComputeStrikePrice(price int64, mode string, value int) (strike *int64, discountPct *int) {
	if price <= 0 {
		return nil, nil
	}
	var s int64
	switch mode {
	case models.StrikeModePercent:
		if value < 1 || value > 90 {
			return nil, nil
		}
		num := price * 100
		den := int64(100-value) * 1000
		s = ((num + den - 1) / den) * 1000
	case models.StrikeModeFixed:
		if value < 1 {
			return nil, nil
		}
		s = price + int64(value)
	default:
		return nil, nil
	}
	if s <= price {
		return nil, nil
	}
	pct := int(((s-price)*200 + s) / (2 * s)) // round((s-price)/s*100)
	return &s, &pct
}

// inPeriod reports whether now is inside [startsAt, endsAt). A nil startsAt
// means "already started"; a nil endsAt means the promo is not valid.
func inPeriod(startsAt, endsAt *time.Time, now time.Time) bool {
	if endsAt == nil || !now.Before(*endsAt) {
		return false
	}
	return startsAt == nil || !now.Before(*startsAt)
}

// Resolve returns the strike price for an item in scope with real price
// price and per-item override o:
//  1. override NONE → no strike price;
//  2. an in-period PERCENT/FIXED override → use it;
//  3. else an in-period global rule for the scope → use it;
//  4. else no strike price.
//
// An out-of-period override falls back to the global rule, so a finished
// item promo doesn't also cancel a running site-wide one.
func (r StrikeRules) Resolve(scope string, price int64, o models.StrikeOverride, now time.Time) models.StrikeDisplay {
	if o.StrikeMode != nil {
		if *o.StrikeMode == models.StrikeModeNone {
			return models.StrikeDisplay{}
		}
		if inPeriod(o.StrikeStartsAt, o.StrikeEndsAt, now) && o.StrikeValue != nil {
			return display(price, *o.StrikeMode, *o.StrikeValue, o.StrikeEndsAt)
		}
	}
	if rule, ok := r[scope]; ok && inPeriod(rule.StartsAt, rule.EndsAt, now) {
		return display(price, rule.Mode, rule.Value, rule.EndsAt)
	}
	return models.StrikeDisplay{}
}

func display(price int64, mode string, value int, endsAt *time.Time) models.StrikeDisplay {
	strike, pct := ComputeStrikePrice(price, mode, value)
	if strike == nil {
		return models.StrikeDisplay{}
	}
	return models.StrikeDisplay{StrikePriceIdr: strike, DiscountPercent: pct, PromoEndsAt: endsAt}
}

// StrikeStatus reports whether a rule/override is off, not yet started,
// running, or finished.
func StrikeStatus(mode string, startsAt, endsAt *time.Time, now time.Time) string {
	switch {
	case mode == "" || mode == models.StrikeModeNone || endsAt == nil:
		return StrikeStatusOff
	case !now.Before(*endsAt):
		return StrikeStatusEnded
	case startsAt != nil && now.Before(*startsAt):
		return StrikeStatusScheduled
	default:
		return StrikeStatusActive
	}
}

// ValidateStrike checks a rule or override before it is saved. NONE needs
// nothing else. PERCENT/FIXED need a value in range and an end date that is
// in the future, after the start, and at most MaxPromoDays after the later
// of the start and now.
func ValidateStrike(mode string, value int, startsAt, endsAt *time.Time, now time.Time) error {
	switch mode {
	case models.StrikeModeNone:
		return nil
	case models.StrikeModePercent:
		if value < 1 || value > 90 {
			return validationErrorf("percent value must be between 1 and 90")
		}
	case models.StrikeModeFixed:
		if value < 1 {
			return validationErrorf("fixed value must be at least 1")
		}
	default:
		return validationErrorf("mode must be NONE, PERCENT or FIXED")
	}
	if endsAt == nil {
		return validationErrorf("ends_at is required for a promo")
	}
	if !endsAt.After(now) {
		return validationErrorf("ends_at must be in the future")
	}
	start := now
	if startsAt != nil {
		if !endsAt.After(*startsAt) {
			return validationErrorf("ends_at must be after starts_at")
		}
		if startsAt.After(now) {
			start = *startsAt
		}
	}
	if endsAt.Sub(start) > MaxPromoDays*24*time.Hour {
		return validationErrorf("a promo can run for at most %d days", MaxPromoDays)
	}
	return nil
}

// StrikeInput is the admin payload for a rule or a per-item override.
// For an override, a nil Mode means "inherit the global rule".
type StrikeInput struct {
	Mode     *string    `json:"strike_mode"`
	Value    *int       `json:"strike_value"`
	StartsAt *time.Time `json:"strike_starts_at"`
	EndsAt   *time.Time `json:"strike_ends_at"`
}

// ToOverride validates the input and turns it into the stored override.
// Inherit (nil mode) and NONE clear the value and period.
func (in StrikeInput) ToOverride(now time.Time) (models.StrikeOverride, error) {
	if in.Mode == nil {
		return models.StrikeOverride{}, nil
	}
	mode := *in.Mode
	if mode == models.StrikeModeNone {
		return models.StrikeOverride{StrikeMode: &mode}, nil
	}
	value := 0
	if in.Value != nil {
		value = *in.Value
	}
	if err := ValidateStrike(mode, value, in.StartsAt, in.EndsAt, now); err != nil {
		return models.StrikeOverride{}, err
	}
	return models.StrikeOverride{
		StrikeMode:     &mode,
		StrikeValue:    &value,
		StrikeStartsAt: in.StartsAt,
		StrikeEndsAt:   in.EndsAt,
	}, nil
}

// StrikeRuleView is a global rule plus its current status (admin use).
type StrikeRuleView struct {
	models.StrikePriceRule
	Status string `json:"status"`
}

// ListRules returns the three global rules with their status.
func (s *StrikePriceService) ListRules() ([]StrikeRuleView, error) {
	rules, err := models.FindAllStrikePriceRules(s.db)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	views := make([]StrikeRuleView, len(rules))
	for i, r := range rules {
		views[i] = StrikeRuleView{StrikePriceRule: r, Status: StrikeStatus(r.Mode, r.StartsAt, r.EndsAt, now)}
	}
	return views, nil
}

// UpdateRuleInput is the admin payload for PUT /admin/strike-price-rules/:scope.
type UpdateRuleInput struct {
	Mode     string     `json:"mode" binding:"required"`
	Value    int        `json:"value"`
	StartsAt *time.Time `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at"`
}

// UpdateRule validates and saves one global rule. Returns
// gorm.ErrRecordNotFound for an unknown scope.
func (s *StrikePriceService) UpdateRule(scope string, in UpdateRuleInput) (*StrikeRuleView, error) {
	switch scope {
	case models.StrikeScopeArCard, models.StrikeScopeDongeng, models.StrikeScopePackage:
	default:
		return nil, gorm.ErrRecordNotFound
	}
	now := time.Now()
	if err := ValidateStrike(in.Mode, in.Value, in.StartsAt, in.EndsAt, now); err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"mode":       in.Mode,
		"value":      in.Value,
		"starts_at":  in.StartsAt,
		"ends_at":    in.EndsAt,
		"updated_at": now,
	}
	if in.Mode == models.StrikeModeNone {
		updates["value"] = 0
		updates["starts_at"] = nil
		updates["ends_at"] = nil
	}
	result := s.db.Model(&models.StrikePriceRule{}).Where("scope = ?", scope).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var rule models.StrikePriceRule
	if err := s.db.Where("scope = ?", scope).First(&rule).Error; err != nil {
		return nil, err
	}
	return &StrikeRuleView{StrikePriceRule: rule, Status: StrikeStatus(rule.Mode, rule.StartsAt, rule.EndsAt, now)}, nil
}
