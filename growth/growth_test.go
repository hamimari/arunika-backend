package growth

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lmsSHA256 must match the app's copy (assets/growth/who2006_lms.csv). If you
// regenerate the file, update the constant here and in the app's test.
const lmsSHA256 = "8e3c98c8f1c1ca812122660c4e51467c937c541238546a2ab6d1a5660177b744"

func TestLMSFileChecksum(t *testing.T) {
	sum := sha256.Sum256(lmsCSV)
	assert.Equal(t, lmsSHA256, hex.EncodeToString(sum[:]))
}

// Every WHO SD line at monthly ages must classify to exactly that z-score.
func TestGoldenFixture(t *testing.T) {
	f, err := os.Open("testdata/golden.csv")
	require.NoError(t, err)
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)
	require.Greater(t, len(rows), 2000)

	for _, r := range rows[1:] {
		day, _ := strconv.Atoi(r[2])
		value, _ := strconv.ParseFloat(r[4], 64)
		want, _ := strconv.ParseFloat(r[5], 64)
		got, ok := ZScore(Indicator(r[0]), Sex(r[1]), day, value)
		require.True(t, ok)
		assert.InDelta(t, want, got, 0.01, "%v", r)
	}
}

func f(v float64) *float64 { return &v }

func date(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestClassify_RFCExample(t *testing.T) {
	res, err := Classify(Input{
		Sex: Male, BirthDate: date("2023-07-12"), MeasuredOn: date("2026-09-12"),
		HeightCm: f(94.8), WeightKg: f(13.9), Position: Standing,
	})
	require.NoError(t, err)
	assert.Equal(t, 1158, res.AgeDays)
	assert.Equal(t, "3 tahun 2 bulan", res.AgeLabel)
	assert.Equal(t, CategoryNormal, res.HFA.Category)
	assert.Equal(t, CategoryNormal, res.WFA.Category)
	assert.False(t, res.Flagged())
}

func TestZScore_BoyAt36Months(t *testing.T) {
	median, _ := ZScore(HeightForAge, Male, 1096, 96.1)
	assert.InDelta(t, 0.0, median, 0.01)
	// 88.7 is WHO's monthly −2 SD line rounded to 0.1 cm, so it lands within
	// 0.02 rather than 0.01; the exact lines are covered by the golden fixture.
	minus2, _ := ZScore(HeightForAge, Male, 1096, 88.7)
	assert.InDelta(t, -2.0, minus2, 0.02)
	assert.Equal(t, CategoryNormal, Category(HeightForAge, minus2))
}

func TestClassify_OnlyWeight(t *testing.T) {
	res, err := Classify(Input{Sex: Female, BirthDate: date("2024-01-01"), MeasuredOn: date("2025-01-01"), WeightKg: f(9), Position: Recumbent})
	require.NoError(t, err)
	assert.Nil(t, res.HFA)
	require.NotNil(t, res.WFA)
}

func TestClassify_BeforeBirth(t *testing.T) {
	_, err := Classify(Input{Sex: Male, BirthDate: date("2025-01-02"), MeasuredOn: date("2025-01-01"), WeightKg: f(3)})
	assert.ErrorIs(t, err, ErrMeasuredBeforeBirth)
}

func TestClassify_OlderThanFiveYears(t *testing.T) {
	birth := date("2020-01-01")
	res, err := Classify(Input{Sex: Male, BirthDate: birth, MeasuredOn: birth.AddDate(0, 0, 1900), HeightCm: f(115), WeightKg: f(20), Position: Standing})
	require.NoError(t, err)
	assert.Nil(t, res.HFA.Z)
	assert.Equal(t, CategoryOutOfRange, res.HFA.Category)
	assert.Equal(t, CategoryOutOfRange, res.WFA.Category)
}

func TestAdjustHeight_AtTheBoundary(t *testing.T) {
	assert.Equal(t, 85.7, AdjustHeight(85, Standing, 730))
	assert.Equal(t, 85.0, AdjustHeight(85, Recumbent, 730))
	assert.Equal(t, 84.3, AdjustHeight(85, Recumbent, 731))
	assert.Equal(t, 85.0, AdjustHeight(85, Standing, 731))

	// The adjusted value is what gets classified: standing at 730 days equals
	// lying 0.7 cm taller.
	birth := date("2023-01-01")
	on := birth.AddDate(0, 0, 730)
	standing, _ := Classify(Input{Sex: Male, BirthDate: birth, MeasuredOn: on, HeightCm: f(85), Position: Standing})
	lying, _ := Classify(Input{Sex: Male, BirthDate: birth, MeasuredOn: on, HeightCm: f(85.7), Position: Recumbent})
	assert.Equal(t, *lying.HFA.Z, *standing.HFA.Z)
	assert.Equal(t, 85.7, *standing.AdjustedHeightCm)
}

func TestZScore_WeightTailRule(t *testing.T) {
	const day = 1096
	sd2, _ := SDValue(WeightForAge, Male, day, 2)
	sd3, _ := SDValue(WeightForAge, Male, day, 3)
	x := sd3 + 0.4*(sd3-sd2)
	z, _ := ZScore(WeightForAge, Male, day, x)
	assert.InDelta(t, 3.4, z, 0.01)

	p := tables[tableKey{WeightForAge, Male}][day]
	plain := (math.Pow(x/p.m, p.l) - 1) / (p.l * p.s)
	assert.NotEqual(t, round(plain, 2), z, "the tail rule must change the raw LMS value")

	sdm2, _ := SDValue(WeightForAge, Male, day, -2)
	sdm3, _ := SDValue(WeightForAge, Male, day, -3)
	low, _ := ZScore(WeightForAge, Male, day, sdm3-0.5*(sdm2-sdm3))
	assert.InDelta(t, -3.5, low, 0.01)
}

func TestCategory_Boundaries(t *testing.T) {
	cases := []struct {
		ind  Indicator
		z    float64
		want string
	}{
		{HeightForAge, -3.01, CategorySeverelyStunted},
		{HeightForAge, -3.00, CategoryStunted},
		{HeightForAge, -2.01, CategoryStunted},
		{HeightForAge, -2.00, CategoryNormal},
		{HeightForAge, 3.00, CategoryNormal},
		{HeightForAge, 3.01, CategoryTall},
		{WeightForAge, -3.01, CategorySeverelyUnderweight},
		{WeightForAge, -3.00, CategoryUnderweight},
		{WeightForAge, -2.00, CategoryNormal},
		{WeightForAge, 1.00, CategoryNormal},
		{WeightForAge, 1.01, CategoryRiskOverweight},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, Category(c.ind, c.z), "%s %v", c.ind, c.z)
	}
}

func TestImplausible(t *testing.T) {
	assert.True(t, Implausible(HeightForAge, -6.01))
	assert.False(t, Implausible(HeightForAge, 6))
	assert.True(t, Implausible(WeightForAge, 5.01))
	assert.False(t, Implausible(WeightForAge, -6))

	res, _ := Classify(Input{Sex: Male, BirthDate: date("2023-07-12"), MeasuredOn: date("2026-09-12"), HeightCm: f(9.5), Position: Standing})
	assert.True(t, res.HFA.Flagged)
	assert.True(t, res.Flagged())
}

func TestNormalizeSex(t *testing.T) {
	for in, want := range map[string]Sex{"Laki-Laki": Male, "laki-laki": Male, "male": Male, "Perempuan": Female, " PEREMPUAN ": Female, "female": Female} {
		got, ok := NormalizeSex(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "lainnya", "x"} {
		_, ok := NormalizeSex(in)
		assert.False(t, ok, in)
	}
}

func TestAgeLabel(t *testing.T) {
	assert.Equal(t, "0 bulan", AgeLabel(0))
	assert.Equal(t, "7 bulan", AgeLabel(214))
	assert.Equal(t, "2 tahun", AgeLabel(731))
	assert.Equal(t, "3 tahun 2 bulan", AgeLabel(1158))
}
