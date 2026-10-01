package services

import (
	"arunika_backend/models"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestComputeStrikePrice(t *testing.T) {
	cases := []struct {
		name       string
		price      int64
		mode       string
		value      int
		wantStrike int64
		wantPct    int
		wantNil    bool
	}{
		{"percent rounds up to nearest thousand", 39000, models.StrikeModePercent, 20, 49000, 20, false},
		{"percent on small price", 15000, models.StrikeModePercent, 20, 19000, 21, false},
		{"percent on package", 79000, models.StrikeModePercent, 20, 99000, 20, false},
		{"fixed adds amount", 29000, models.StrikeModeFixed, 10000, 39000, 26, false},
		{"none", 29000, models.StrikeModeNone, 0, 0, 0, true},
		{"zero price", 0, models.StrikeModeFixed, 10000, 0, 0, true},
		{"percent out of range", 29000, models.StrikeModePercent, 95, 0, 0, true},
		{"fixed zero", 29000, models.StrikeModeFixed, 0, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			strike, pct := ComputeStrikePrice(tc.price, tc.mode, tc.value)
			if tc.wantNil {
				assert.Nil(t, strike)
				assert.Nil(t, pct)
				return
			}
			require.NotNil(t, strike)
			require.NotNil(t, pct)
			assert.Equal(t, tc.wantStrike, *strike)
			assert.Equal(t, tc.wantPct, *pct)
		})
	}
}

func TestStrikeRules_Resolve(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)
	farFuture := now.Add(48 * time.Hour)

	rules := StrikeRules{
		models.StrikeScopeDongeng: {Scope: models.StrikeScopeDongeng, Mode: models.StrikeModePercent, Value: 20, EndsAt: &future},
		models.StrikeScopeArCard:  {Scope: models.StrikeScopeArCard, Mode: models.StrikeModePercent, Value: 20, EndsAt: &future},
		models.StrikeScopePackage: {Scope: models.StrikeScopePackage, Mode: models.StrikeModeFixed, Value: 10000, EndsAt: &future},
	}

	t.Run("item inherits global rule", func(t *testing.T) {
		d := rules.Resolve(models.StrikeScopeDongeng, 39000, models.StrikeOverride{}, now)
		require.NotNil(t, d.StrikePriceIdr)
		assert.Equal(t, int64(49000), *d.StrikePriceIdr)
		assert.Equal(t, future, *d.PromoEndsAt)
	})

	t.Run("active override wins over global rule", func(t *testing.T) {
		o := models.StrikeOverride{StrikeMode: strPtr(models.StrikeModeFixed), StrikeValue: intPtr(5000), StrikeEndsAt: &farFuture}
		d := rules.Resolve(models.StrikeScopeArCard, 15000, o, now)
		require.NotNil(t, d.StrikePriceIdr)
		assert.Equal(t, int64(20000), *d.StrikePriceIdr)
		assert.Equal(t, farFuture, *d.PromoEndsAt)
	})

	t.Run("expired override falls back to global rule", func(t *testing.T) {
		o := models.StrikeOverride{StrikeMode: strPtr(models.StrikeModeFixed), StrikeValue: intPtr(5000), StrikeEndsAt: &past}
		d := rules.Resolve(models.StrikeScopeArCard, 15000, o, now)
		require.NotNil(t, d.StrikePriceIdr)
		assert.Equal(t, int64(19000), *d.StrikePriceIdr)
	})

	t.Run("override NONE opts out", func(t *testing.T) {
		o := models.StrikeOverride{StrikeMode: strPtr(models.StrikeModeNone)}
		d := rules.Resolve(models.StrikeScopePackage, 79000, o, now)
		assert.Nil(t, d.StrikePriceIdr)
		assert.Nil(t, d.DiscountPercent)
		assert.Nil(t, d.PromoEndsAt)
	})

	t.Run("rule not started yet", func(t *testing.T) {
		r := StrikeRules{models.StrikeScopePackage: {Mode: models.StrikeModeFixed, Value: 10000, StartsAt: &future, EndsAt: &farFuture}}
		assert.Nil(t, r.Resolve(models.StrikeScopePackage, 79000, models.StrikeOverride{}, now).StrikePriceIdr)
	})

	t.Run("rule ends exactly at ends_at", func(t *testing.T) {
		r := StrikeRules{models.StrikeScopePackage: {Mode: models.StrikeModeFixed, Value: 10000, EndsAt: &now}}
		assert.Nil(t, r.Resolve(models.StrikeScopePackage, 79000, models.StrikeOverride{}, now).StrikePriceIdr)
	})

	t.Run("nil rules still honour an override", func(t *testing.T) {
		var none StrikeRules
		o := models.StrikeOverride{StrikeMode: strPtr(models.StrikeModePercent), StrikeValue: intPtr(20), StrikeEndsAt: &future}
		d := none.Resolve(models.StrikeScopeDongeng, 39000, o, now)
		require.NotNil(t, d.StrikePriceIdr)
		assert.Equal(t, int64(49000), *d.StrikePriceIdr)
	})
}

func TestValidateStrike(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	in30 := now.AddDate(0, 0, 30)
	in91 := now.AddDate(0, 0, 91)
	past := now.Add(-time.Hour)
	startIn10 := now.AddDate(0, 0, 10)
	startIn10Plus90 := startIn10.AddDate(0, 0, 90)

	assert.NoError(t, ValidateStrike(models.StrikeModeNone, 0, nil, nil, now))
	assert.NoError(t, ValidateStrike(models.StrikeModePercent, 20, nil, &in30, now))
	assert.NoError(t, ValidateStrike(models.StrikeModeFixed, 10000, &startIn10, &startIn10Plus90, now), "90 days counted from a future start")

	for name, err := range map[string]error{
		"percent too high":  ValidateStrike(models.StrikeModePercent, 95, nil, &in30, now),
		"percent zero":      ValidateStrike(models.StrikeModePercent, 0, nil, &in30, now),
		"fixed zero":        ValidateStrike(models.StrikeModeFixed, 0, nil, &in30, now),
		"unknown mode":      ValidateStrike("HALF", 10, nil, &in30, now),
		"missing end":       ValidateStrike(models.StrikeModeFixed, 1000, nil, nil, now),
		"end in past":       ValidateStrike(models.StrikeModeFixed, 1000, nil, &past, now),
		"end before start":  ValidateStrike(models.StrikeModeFixed, 1000, &in30, &startIn10, now),
		"longer than 90 d.": ValidateStrike(models.StrikeModePercent, 20, nil, &in91, now),
	} {
		assert.True(t, IsValidationError(err), name)
	}
}

func TestStrikeInput_ToOverride(t *testing.T) {
	now := time.Now()
	end := now.AddDate(0, 0, 7)

	o, err := StrikeInput{}.ToOverride(now)
	require.NoError(t, err)
	assert.Nil(t, o.StrikeMode, "nil mode means inherit")

	o, err = StrikeInput{Mode: strPtr(models.StrikeModeNone), Value: intPtr(5), EndsAt: &end}.ToOverride(now)
	require.NoError(t, err)
	assert.Equal(t, models.StrikeModeNone, *o.StrikeMode)
	assert.Nil(t, o.StrikeValue, "NONE clears the value and period")
	assert.Nil(t, o.StrikeEndsAt)

	_, err = StrikeInput{Mode: strPtr(models.StrikeModeFixed), Value: intPtr(5000)}.ToOverride(now)
	assert.True(t, IsValidationError(err), "end date required")
}

func TestStrikeStatus(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	farFuture := now.Add(2 * time.Hour)
	assert.Equal(t, StrikeStatusOff, StrikeStatus(models.StrikeModeNone, nil, nil, now))
	assert.Equal(t, StrikeStatusActive, StrikeStatus(models.StrikeModeFixed, nil, &future, now))
	assert.Equal(t, StrikeStatusScheduled, StrikeStatus(models.StrikeModeFixed, &future, &farFuture, now))
	assert.Equal(t, StrikeStatusEnded, StrikeStatus(models.StrikeModeFixed, nil, &past, now))
}

func strikeRuleCols() []string {
	return []string{"scope", "mode", "value", "starts_at", "ends_at", "updated_at"}
}

func TestStrikePriceService_LoadRules(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewStrikePriceService(db)
	end := time.Now().Add(time.Hour)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "strike_price_rules" ORDER BY scope ASC`)).
		WillReturnRows(sqlmock.NewRows(strikeRuleCols()).
			AddRow("PACKAGE", "PERCENT", 25, nil, end, time.Now()))

	rules, err := svc.LoadRules()
	require.NoError(t, err)
	assert.Equal(t, 25, rules[models.StrikeScopePackage].Value)
	require.NoError(t, mock.ExpectationsWereMet())

	var nilSvc *StrikePriceService
	rules, err = nilSvc.LoadRules()
	require.NoError(t, err)
	assert.Nil(t, rules)
}

func TestStrikePriceService_UpdateRule_UnknownScope(t *testing.T) {
	db, _ := setupMockDB(t)
	_, err := NewStrikePriceService(db).UpdateRule("BUNDLE", UpdateRuleInput{Mode: models.StrikeModeNone})
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestStrikePriceService_UpdateRule_Invalid(t *testing.T) {
	db, mock := setupMockDB(t)
	end := time.Now().AddDate(0, 0, 120)
	_, err := NewStrikePriceService(db).UpdateRule(models.StrikeScopePackage,
		UpdateRuleInput{Mode: models.StrikeModePercent, Value: 20, EndsAt: &end})
	assert.True(t, IsValidationError(err))
	require.NoError(t, mock.ExpectationsWereMet(), "nothing written on a validation error")
}

// GET /premium/packs: packages carry the computed strike price and the
// public response hides how the override was configured.
func TestPremiumPackService_GetActivePacks_AppliesStrikePrice(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	strike := NewStrikePriceService(gormDB)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB))).WithStrikePricing(strike)
	now := time.Now()
	ruleEnd := now.AddDate(0, 0, 10)
	overrideEnd := now.AddDate(0, 0, 3)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE is_active = true ORDER BY sort_order asc`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "price_idr", "type", "strike_mode", "strike_value", "strike_starts_at", "strike_ends_at"}).
			AddRow("p1", "Paket Hutan", 79000, "content", nil, nil, nil, nil).
			AddRow("p2", "Tahunan", 299000, "subscription", "FIXED", 100000, nil, overrideEnd).
			AddRow("p3", "Bulanan", 39000, "subscription", "NONE", nil, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "strike_price_rules" ORDER BY scope ASC`)).
		WillReturnRows(sqlmock.NewRows(strikeRuleCols()).
			AddRow("PACKAGE", "PERCENT", 20, nil, ruleEnd, now))

	packs, err := svc.GetActivePacks("", nil)
	require.NoError(t, err)
	require.Len(t, packs, 3)

	require.NotNil(t, packs[0].StrikePriceIdr, "inherits the PACKAGE rule")
	assert.Equal(t, int64(99000), *packs[0].StrikePriceIdr)
	assert.Equal(t, 20, *packs[0].DiscountPercent)
	assert.Equal(t, ruleEnd.Unix(), packs[0].PromoEndsAt.Unix())

	require.NotNil(t, packs[1].StrikePriceIdr, "own override wins")
	assert.Equal(t, int64(399000), *packs[1].StrikePriceIdr)
	assert.Nil(t, packs[1].StrikeMode, "override config is not exposed publicly")

	assert.Nil(t, packs[2].StrikePriceIdr, "NONE opts out")
	assert.NoError(t, mock.ExpectationsWereMet())
}
