// Package growth classifies a child's height and weight against the WHO Child
// Growth Standards (2006, 0-5 years) with the LMS method, and maps z-scores to
// the anthropometric categories of Permenkes No. 2 Tahun 2020.
//
// The same engine runs in the app (lib/core/growth/) for the live form preview
// and the chart. Both load a byte-identical data/who2006_lms.csv and must pass
// testdata/golden.csv, so the server (the source of truth for stored
// categories) and the app never disagree. Regenerate both files with
// scripts/build_who_lms.py.
package growth

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

//go:embed data/who2006_lms.csv
var lmsCSV []byte

// WHOVersion identifies the reference the stored z-scores were computed with.
const WHOVersion = "who2006"

// MaxAgeDays is the last day covered by the WHO 0-5 year standard. Older
// measurements are stored without a z-score.
const MaxAgeDays = 1856

// lengthHeightCutoffDays: below this age the reference is length (lying),
// from it onwards height (standing).
const lengthHeightCutoffDays = 731

type Sex string

const (
	Male   Sex = "male"
	Female Sex = "female"
)

type Position string

const (
	Standing  Position = "standing"
	Recumbent Position = "recumbent"
)

type Indicator string

const (
	HeightForAge Indicator = "lhfa"
	WeightForAge Indicator = "wfa"
)

// Categories. "out_of_range" is used for both indicators past MaxAgeDays.
const (
	CategorySeverelyStunted     = "severely_stunted"
	CategoryStunted             = "stunted"
	CategoryNormal              = "normal"
	CategoryTall                = "tall"
	CategorySeverelyUnderweight = "severely_underweight"
	CategoryUnderweight         = "underweight"
	CategoryRiskOverweight      = "risk_overweight"
	CategoryOutOfRange          = "out_of_range"
)

var ErrMeasuredBeforeBirth = errors.New("measured before birth")

type lms struct{ l, m, s float64 }

type tableKey struct {
	indicator Indicator
	sex       Sex
}

var tables = mustLoad(lmsCSV)

func mustLoad(data []byte) map[tableKey][]lms {
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		panic(fmt.Sprintf("growth: read LMS file: %v", err))
	}
	out := map[tableKey][]lms{}
	for _, r := range rows[1:] {
		key := tableKey{Indicator(r[0]), Sex(r[1])}
		day, err := strconv.Atoi(r[2])
		if err != nil || day != len(out[key]) {
			panic(fmt.Sprintf("growth: LMS rows for %v out of order at day %s", key, r[2]))
		}
		var v [3]float64
		for i := range v {
			if v[i], err = strconv.ParseFloat(r[3+i], 64); err != nil {
				panic(fmt.Sprintf("growth: bad LMS value %q", r[3+i]))
			}
		}
		out[key] = append(out[key], lms{v[0], v[1], v[2]})
	}
	for _, ind := range []Indicator{HeightForAge, WeightForAge} {
		for _, sex := range []Sex{Male, Female} {
			if n := len(out[tableKey{ind, sex}]); n != MaxAgeDays+1 {
				panic(fmt.Sprintf("growth: %s/%s has %d days, want %d", ind, sex, n, MaxAgeDays+1))
			}
		}
	}
	return out
}

// NormalizeSex maps the stored children.gender value ("Laki-Laki" /
// "Perempuan" from the app, or English) to a Sex. ok is false for anything
// else, which callers treat as an incomplete profile.
func NormalizeSex(gender string) (Sex, bool) {
	switch strings.ToLower(strings.TrimSpace(gender)) {
	case "laki-laki", "laki laki", "male", "l", "m":
		return Male, true
	case "perempuan", "female", "p", "f":
		return Female, true
	}
	return "", false
}

// AgeDays is the whole number of days between two calendar dates. Times of day
// and time zones are ignored.
func AgeDays(birthDate, measuredOn time.Time) int {
	b := time.Date(birthDate.Year(), birthDate.Month(), birthDate.Day(), 0, 0, 0, 0, time.UTC)
	m := time.Date(measuredOn.Year(), measuredOn.Month(), measuredOn.Day(), 0, 0, 0, 0, time.UTC)
	return int(m.Sub(b).Hours() / 24)
}

// AgeMonths is the number of completed months, as WHO counts them.
func AgeMonths(ageDays int) int { return int(math.Floor(float64(ageDays) / 30.4375)) }

// AgeLabel formats an age for parents: "3 tahun 2 bulan", "7 bulan", "2 tahun".
func AgeLabel(ageDays int) string {
	months := AgeMonths(ageDays)
	y, m := months/12, months%12
	switch {
	case y == 0:
		return fmt.Sprintf("%d bulan", m)
	case m == 0:
		return fmt.Sprintf("%d tahun", y)
	}
	return fmt.Sprintf("%d tahun %d bulan", y, m)
}

// AdjustHeight converts a measurement taken in the "wrong" position for the
// child's age to the reference one: +0.7 cm for a standing child under 731
// days, -0.7 cm for a lying child from 731 days.
func AdjustHeight(heightCm float64, position Position, ageDays int) float64 {
	switch {
	case ageDays < lengthHeightCutoffDays && position == Standing:
		return round(heightCm+0.7, 1)
	case ageDays >= lengthHeightCutoffDays && position == Recumbent:
		return round(heightCm-0.7, 1)
	}
	return heightCm
}

// ZScore returns the z-score of x for an indicator, sex and age, rounded to 2
// decimals. Weight-for-age beyond +-3 uses WHO's restricted LMS tail rule.
// ok is false when the age is outside 0..MaxAgeDays.
func ZScore(ind Indicator, sex Sex, ageDays int, x float64) (z float64, ok bool) {
	t := tables[tableKey{ind, sex}]
	if ageDays < 0 || ageDays >= len(t) || t == nil {
		return 0, false
	}
	p := t[ageDays]
	z = (math.Pow(x/p.m, p.l) - 1) / (p.l * p.s)
	if ind == WeightForAge && (z > 3 || z < -3) {
		sd := func(k float64) float64 { return p.m * math.Pow(1+p.l*p.s*k, 1/p.l) }
		if z > 3 {
			z = 3 + (x-sd(3))/(sd(3)-sd(2))
		} else {
			z = -3 + (x-sd(-3))/(sd(-2)-sd(-3))
		}
	}
	return round(z, 2), true
}

// SDValue is the measurement at z-score k (the WHO SD line), used to draw the
// chart zones. Weight beyond +-3 is extended linearly, as WHO's tables do.
func SDValue(ind Indicator, sex Sex, ageDays int, k float64) (float64, bool) {
	t := tables[tableKey{ind, sex}]
	if ageDays < 0 || ageDays >= len(t) {
		return 0, false
	}
	p := t[ageDays]
	sd := func(k float64) float64 { return p.m * math.Pow(1+p.l*p.s*k, 1/p.l) }
	if ind == WeightForAge && k > 3 {
		return sd(3) + (k-3)*(sd(3)-sd(2)), true
	}
	if ind == WeightForAge && k < -3 {
		return sd(-3) + (k+3)*(sd(-2)-sd(-3)), true
	}
	return sd(k), true
}

// Category maps a rounded z-score to its Permenkes 2/2020 category.
func Category(ind Indicator, z float64) string {
	switch {
	case z < -3:
		if ind == HeightForAge {
			return CategorySeverelyStunted
		}
		return CategorySeverelyUnderweight
	case z < -2:
		if ind == HeightForAge {
			return CategoryStunted
		}
		return CategoryUnderweight
	case ind == HeightForAge && z > 3:
		return CategoryTall
	case ind == WeightForAge && z > 1:
		return CategoryRiskOverweight
	}
	return CategoryNormal
}

// Implausible reports whether z is outside WHO's flag limits.
func Implausible(ind Indicator, z float64) bool {
	if ind == HeightForAge {
		return z < -6 || z > 6
	}
	return z < -6 || z > 5
}

type Input struct {
	Sex        Sex
	BirthDate  time.Time
	MeasuredOn time.Time
	HeightCm   *float64
	WeightKg   *float64
	Position   Position
}

// IndicatorResult has a nil Z when the age is past MaxAgeDays.
type IndicatorResult struct {
	Z        *float64
	Category string
	Flagged  bool
}

type Result struct {
	AgeDays          int
	AgeLabel         string
	AdjustedHeightCm *float64
	HFA              *IndicatorResult // nil when no height was given
	WFA              *IndicatorResult // nil when no weight was given
}

// Flagged reports whether either indicator is implausible.
func (r Result) Flagged() bool {
	return (r.HFA != nil && r.HFA.Flagged) || (r.WFA != nil && r.WFA.Flagged)
}

// Classify computes age, z-scores and categories for one measurement.
func Classify(in Input) (Result, error) {
	age := AgeDays(in.BirthDate, in.MeasuredOn)
	if age < 0 {
		return Result{}, ErrMeasuredBeforeBirth
	}
	res := Result{AgeDays: age, AgeLabel: AgeLabel(age)}
	if in.HeightCm != nil {
		h := AdjustHeight(*in.HeightCm, in.Position, age)
		res.AdjustedHeightCm = &h
		res.HFA = indicatorResult(HeightForAge, in.Sex, age, h)
	}
	if in.WeightKg != nil {
		res.WFA = indicatorResult(WeightForAge, in.Sex, age, *in.WeightKg)
	}
	return res, nil
}

func indicatorResult(ind Indicator, sex Sex, age int, x float64) *IndicatorResult {
	z, ok := ZScore(ind, sex, age, x)
	if !ok {
		return &IndicatorResult{Category: CategoryOutOfRange}
	}
	return &IndicatorResult{Z: &z, Category: Category(ind, z), Flagged: Implausible(ind, z)}
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
