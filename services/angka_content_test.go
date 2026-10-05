package services

import (
	"testing"

	"arunika_backend/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestStarsFor(t *testing.T) {
	s := models.AngkaStars{Three: 9, Two: 7}
	for first, want := range map[int]int{10: 3, 9: 3, 8: 2, 7: 2, 6: 1, 0: 1} {
		assert.Equal(t, want, starsFor(first, s), "first-try correct %d", first)
	}
}

func TestPrerequisiteCycle(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	// Published: B requires A; C requires B.
	prereq := map[uuid.UUID]*uuid.UUID{a: nil, b: &a, c: &b}
	assert.True(t, prerequisiteCycle(a, c, prereq), "A requiring C closes A → C → B → A")
	assert.False(t, prerequisiteCycle(c, b, prereq))
}

func TestValidateLevelShapeAndRules(t *testing.T) {
	obj := models.AngkaObject{ID: uuid.New(), Status: models.AngkaStatusPublished, Content: models.JSONB(`{}`)}
	hidden := models.AngkaObject{ID: uuid.New(), Status: models.AngkaStatusHidden, Content: models.JSONB(`{}`)}
	chk := levelChecks{
		objects:       map[uuid.UUID]models.AngkaObject{obj.ID: obj, hidden.ID: hidden},
		prerequisites: map[uuid.UUID]*uuid.UUID{},
		countAudio:    map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true},
	}
	c := defaultLevelContent("Hitung 1 sampai 5", nil)
	c.ObjectIDs = []uuid.UUID{obj.ID}
	assert.Empty(t, validateLevel(uuid.New(), c, chk))

	c.ObjectIDs = []uuid.UUID{obj.ID, hidden.ID}
	c.Range = models.AngkaRange{Min: 4, Max: 3}
	c.Stars = models.AngkaStars{Three: 11, Two: 2}
	codes := map[string]string{}
	for _, f := range validateLevel(uuid.New(), c, chk) {
		codes[f.Path] = f.Code
	}
	assert.Equal(t, "NOT_PUBLISHED", codes["object_ids"])
	assert.Equal(t, "LESS_THAN_MIN", codes["range.max"])
	assert.Equal(t, "GREATER_THAN_QUESTIONS", codes["stars.three"])
}

func TestNormalizeLevelDropsDuplicateObjects(t *testing.T) {
	a := uuid.New()
	c := models.AngkaLevelContent{Name: "  Level  ", ObjectIDs: []uuid.UUID{a, a}}
	normalizeLevel(&c)
	assert.Equal(t, "Level", c.Name)
	assert.Equal(t, []uuid.UUID{a}, c.ObjectIDs)
}
