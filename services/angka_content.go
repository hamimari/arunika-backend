package services

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"arunika_backend/angkagen"
	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Belajar Angka errors.
var (
	ErrAngkaObjectNotFound  = errors.New("object not found")
	ErrAngkaNumberNotFound  = errors.New("number not found")
	ErrAngkaLevelNotFound   = errors.New("level not found")
	ErrAngkaSessionNotFound = errors.New("session not found")
	// ErrLevelLocked means the level's prerequisite isn't completed (409).
	ErrLevelLocked = errors.New("complete the previous level first")
	// ErrInvalidTry rejects a try that doesn't fit the session (422).
	ErrInvalidTry = errors.New("try does not fit this session")
	// ErrSessionIncomplete means complete was called before every question
	// was finished (422).
	ErrSessionIncomplete = errors.New("session has unfinished questions")
	// ErrObjectInUse refuses deleting an object a level references (409).
	ErrObjectInUse = errors.New("object is used by a level")
	// ErrObjectLastInLevel refuses hiding a published level's last visible
	// object (409).
	ErrObjectLastInLevel = errors.New("object is the last visible object of a published level")
	// ErrLevelInUse refuses deleting a published or required level (409).
	ErrLevelInUse = errors.New("level was published or is another level's prerequisite")
)

// Publish limits (PRD BO-2, BO-3; design D6).
const (
	angkaMaxNumber    = 20
	angkaMinQuestions = 5
	angkaMaxQuestions = 20
	// Wrong tries kept per question; after that only the right answer is
	// stored, so a child can always finish.
	angkaMaxWrongTries    = 50
	angkaObjectNameRunes  = 30
	angkaQuestionRunes    = 80
	angkaNumberNameRunes  = 20
	angkaLevelNameRunes   = 40
	angkaObjectImageBytes = 300 * 1024
	angkaQuestionAudioMs  = 5000
)

// Default level feedback; {benda} becomes the question's object name.
const (
	AngkaDefaultSuccess = "Hebat! Benar!"
	AngkaDefaultRetry   = "Hampir benar!"
	AngkaDefaultHint    = "Sentuh {benda} satu per satu sambil menyebut 1, 2, 3…"
)

func defaultLevelContent(name string, prerequisite *uuid.UUID) models.AngkaLevelContent {
	return models.AngkaLevelContent{
		Name:           name,
		PrerequisiteID: prerequisite,
		Range:          models.AngkaRange{Min: 1, Max: 5},
		QuestionCount:  10,
		ObjectIDs:      []uuid.UUID{},
		Layout:         angkagen.LayoutScatter,
		Stars:          models.AngkaStars{Three: 9, Two: 7},
		Feedback: models.AngkaLevelFeedback{
			Success: AngkaDefaultSuccess, Retry: AngkaDefaultRetry, Hint: AngkaDefaultHint,
		},
	}
}

func parseObject(raw []byte) models.AngkaObjectContent {
	var c models.AngkaObjectContent
	_ = json.Unmarshal(raw, &c)
	return c
}

func parseNumber(raw []byte) models.AngkaNumberContent {
	var c models.AngkaNumberContent
	_ = json.Unmarshal(raw, &c)
	return c
}

func parseLevel(raw []byte) models.AngkaLevelContent {
	var c models.AngkaLevelContent
	_ = json.Unmarshal(raw, &c)
	if c.ObjectIDs == nil {
		c.ObjectIDs = []uuid.UUID{}
	}
	return c
}

// sameJSON compares two stored documents by value through their typed shape.
func sameJSON[T any](a, b []byte) bool {
	if len(b) == 0 {
		return false
	}
	var x, y T
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return string(a) == string(b)
	}
	ja, _ := json.Marshal(x)
	jb, _ := json.Marshal(y)
	return string(ja) == string(jb)
}

func trimMedia(id *uuid.UUID, u *string) {
	*u = strings.TrimSpace(*u)
	if id != nil {
		*u = ""
	}
}

func normalizeObject(c *models.AngkaObjectContent) {
	c.Name = strings.TrimSpace(c.Name)
	c.QuestionText = strings.TrimSpace(c.QuestionText)
	trimMedia(c.ImageAssetID, &c.ImageURL)
	trimMedia(c.QuestionAudioID, &c.QuestionAudioURL)
}

func normalizeNumber(c *models.AngkaNumberContent) {
	c.Name = strings.TrimSpace(c.Name)
	trimMedia(c.AudioAssetID, &c.AudioURL)
}

func normalizeLevel(c *models.AngkaLevelContent) {
	c.Name = strings.TrimSpace(c.Name)
	if c.ObjectIDs == nil {
		c.ObjectIDs = []uuid.UUID{}
	}
	seen := map[uuid.UUID]bool{}
	ids := c.ObjectIDs[:0]
	for _, id := range c.ObjectIDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	c.ObjectIDs = ids
}

func objectAssetIDs(c models.AngkaObjectContent) []uuid.UUID {
	var ids []uuid.UUID
	for _, id := range []*uuid.UUID{c.ImageAssetID, c.QuestionAudioID} {
		if id != nil {
			ids = append(ids, *id)
		}
	}
	return ids
}

// fieldErrors collects publish validation failures.
type fieldErrors []FieldError

func (f *fieldErrors) add(path, code string) { *f = append(*f, FieldError{Path: path, Code: code}) }

func (f *fieldErrors) text(path, v string, max int) {
	switch {
	case strings.TrimSpace(v) == "":
		f.add(path, "REQUIRED")
	case utf8.RuneCountInString(v) > max:
		f.add(path, "TOO_LONG")
	}
}

// mediaLimits are extra checks on an uploaded asset; zero means none.
type mediaLimits struct {
	maxBytes    int
	maxMs       int
	transparent bool
}

// media checks one slot: an uploaded asset of kind within limits, or else an
// https URL (which can't be measured, so it passes the limits).
func (f *fieldErrors) media(path, urlPath string, id *uuid.UUID, external, kind string,
	assets map[uuid.UUID]AssetView, lim mediaLimits) {
	if id == nil {
		switch {
		case external == "":
			f.add(path, "REQUIRED")
		case !validMediaURL(external):
			f.add(urlPath, "INVALID_URL")
		}
		return
	}
	a, ok := assets[*id]
	switch {
	case !ok:
		f.add(path, "NOT_FOUND")
	case a.Kind != kind:
		f.add(path, "WRONG_KIND")
	case lim.maxBytes > 0 && a.Bytes > lim.maxBytes:
		f.add(path, "TOO_LARGE")
	case lim.maxMs > 0 && a.DurationMs != nil && *a.DurationMs > lim.maxMs:
		f.add(path, "TOO_LONG")
	case lim.transparent && a.HasAlpha != nil && !*a.HasAlpha:
		f.add(path, "NOT_TRANSPARENT")
	}
}

func validateObject(c models.AngkaObjectContent, assets map[uuid.UUID]AssetView) []FieldError {
	var f fieldErrors
	f.text("name", c.Name, angkaObjectNameRunes)
	f.text("question_text", c.QuestionText, angkaQuestionRunes)
	f.media("image_asset_id", "image_url", c.ImageAssetID, c.ImageURL, models.AssetKindImage, assets,
		mediaLimits{maxBytes: angkaObjectImageBytes, transparent: true})
	f.media("question_audio_id", "question_audio_url", c.QuestionAudioID, c.QuestionAudioURL, models.AssetKindAudio, assets,
		mediaLimits{maxMs: angkaQuestionAudioMs})
	return f
}

// visibleObject reports whether an object can be used in new questions.
func visibleObject(o models.AngkaObject) bool {
	return o.Status == models.AngkaStatusPublished && len(o.Content) > 0
}

func validateNumber(c models.AngkaNumberContent, assets map[uuid.UUID]AssetView, objects map[uuid.UUID]models.AngkaObject) []FieldError {
	var f fieldErrors
	f.text("name", c.Name, angkaNumberNameRunes)
	f.media("audio_asset_id", "audio_url", c.AudioAssetID, c.AudioURL, models.AssetKindAudio, assets, mediaLimits{})
	if c.ObjectID == nil {
		f.add("object_id", "REQUIRED")
	} else if o, ok := objects[*c.ObjectID]; !ok || !visibleObject(o) {
		f.add("object_id", "NOT_PUBLISHED")
	}
	return f
}

// levelChecks is what level validation needs from the database.
type levelChecks struct {
	objects map[uuid.UUID]models.AngkaObject
	// prerequisites maps each published level to its published prerequisite.
	prerequisites map[uuid.UUID]*uuid.UUID
	// countAudio holds the values 1–20 that have published audio.
	countAudio map[int]bool
}

// validateLevelShape checks the rules that need no database: what preview
// requires to generate questions.
func validateLevelShape(c models.AngkaLevelContent) []FieldError {
	var f fieldErrors
	if c.Range.Min < 1 || c.Range.Min > angkaMaxNumber {
		f.add("range.min", "OUT_OF_RANGE")
	}
	if c.Range.Max < 1 || c.Range.Max > angkaMaxNumber {
		f.add("range.max", "OUT_OF_RANGE")
	} else if c.Range.Max < c.Range.Min {
		f.add("range.max", "LESS_THAN_MIN")
	}
	if c.QuestionCount < angkaMinQuestions || c.QuestionCount > angkaMaxQuestions {
		f.add("question_count", "OUT_OF_RANGE")
	}
	if c.Layout != angkagen.LayoutScatter && c.Layout != angkagen.LayoutRows {
		f.add("layout", "INVALID")
	}
	return f
}

func validateLevel(id uuid.UUID, c models.AngkaLevelContent, chk levelChecks) []FieldError {
	f := fieldErrors(validateLevelShape(c))
	f.text("name", c.Name, angkaLevelNameRunes)
	if len(c.ObjectIDs) == 0 {
		f.add("object_ids", "REQUIRED")
	}
	for _, oid := range c.ObjectIDs {
		if o, ok := chk.objects[oid]; !ok || !visibleObject(o) {
			f.add("object_ids", "NOT_PUBLISHED")
			break
		}
	}
	switch {
	case c.Stars.Two < 1:
		f.add("stars.two", "OUT_OF_RANGE")
	case c.Stars.Two > c.Stars.Three:
		f.add("stars.two", "GREATER_THAN_THREE")
		f.add("stars.three", "LESS_THAN_TWO")
	}
	if c.Stars.Three > c.QuestionCount {
		f.add("stars.three", "GREATER_THAN_QUESTIONS")
	}
	f.text("feedback.success", c.Feedback.Success, maxFeedbackRunes)
	f.text("feedback.retry", c.Feedback.Retry, maxFeedbackRunes)
	f.text("feedback.hint", c.Feedback.Hint, maxHintRunes)
	if p := c.PrerequisiteID; p != nil {
		if _, ok := chk.prerequisites[*p]; !ok || *p == id {
			f.add("prerequisite_id", "NOT_PUBLISHED")
		} else if prerequisiteCycle(id, *p, chk.prerequisites) {
			f.add("prerequisite_id", "CYCLE")
		}
	}
	if c.Range.Max >= 1 && c.Range.Max <= angkaMaxNumber {
		missing := false
		for v := 1; v <= c.Range.Max; v++ {
			if !chk.countAudio[v] {
				// One error per number, so the backoffice can say which ones.
				f.add("numbers."+strconv.Itoa(v), "NUMBER_AUDIO_MISSING")
				missing = true
			}
		}
		if missing {
			f.add("range.max", "NUMBER_AUDIO_MISSING")
		}
	}
	return f
}

// prerequisiteCycle follows published prerequisites from start and reports
// whether the chain comes back to id.
func prerequisiteCycle(id, start uuid.UUID, prereq map[uuid.UUID]*uuid.UUID) bool {
	cur := &start
	for steps := 0; cur != nil && steps <= len(prereq); steps++ {
		if *cur == id {
			return true
		}
		cur = prereq[*cur]
	}
	return false
}

// AngkaObjectView is an object as the app renders it.
type AngkaObjectView struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	QuestionText     string    `json:"question_text"`
	ImageURL         string    `json:"image_url"`
	QuestionAudioURL string    `json:"question_audio_url"`
	Hidden           bool      `json:"hidden"`
}

// objectViews resolves objects (in ids order) from their published copy, or
// from the draft when useDraft is set and nothing is published yet (preview).
func objectViews(tx *gorm.DB, as *AssetService, ids []uuid.UUID, useDraft bool) ([]AngkaObjectView, error) {
	if len(ids) == 0 {
		return []AngkaObjectView{}, nil
	}
	var rows []models.AngkaObject
	if err := tx.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]models.AngkaObject{}
	contents := map[uuid.UUID]models.AngkaObjectContent{}
	var assetIDs []uuid.UUID
	for _, r := range rows {
		raw := r.Content
		if len(raw) == 0 {
			if !useDraft {
				continue
			}
			raw = r.Draft
		}
		byID[r.ID] = r
		c := parseObject(raw)
		contents[r.ID] = c
		assetIDs = append(assetIDs, objectAssetIDs(c)...)
	}
	assets, err := as.Find(tx, assetIDs)
	if err != nil {
		return nil, err
	}
	out := make([]AngkaObjectView, 0, len(ids))
	for _, id := range ids {
		c, ok := contents[id]
		if !ok {
			continue
		}
		out = append(out, AngkaObjectView{
			ID: id, Name: c.Name, QuestionText: c.QuestionText,
			ImageURL:         mediaURL(assets, c.ImageAssetID, c.ImageURL),
			QuestionAudioURL: mediaURL(assets, c.QuestionAudioID, c.QuestionAudioURL),
			Hidden:           byID[id].Status == models.AngkaStatusHidden,
		})
	}
	return out, nil
}

// countAudio returns the published audio URL of each number 1..max, hidden
// numbers included, keyed by the number as a string.
func countAudio(tx *gorm.DB, as *AssetService, max int) (map[string]string, error) {
	var rows []models.AngkaNumber
	if err := tx.Where("value BETWEEN 1 AND ? AND content IS NOT NULL", max).
		Order("value ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	contents := make([]models.AngkaNumberContent, len(rows))
	var ids []uuid.UUID
	for i, r := range rows {
		contents[i] = parseNumber(r.Content)
		if id := contents[i].AudioAssetID; id != nil {
			ids = append(ids, *id)
		}
	}
	assets, err := as.Find(tx, ids)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for i, r := range rows {
		if u := mediaURL(assets, contents[i].AudioAssetID, contents[i].AudioURL); u != "" {
			out[strconv.Itoa(r.Value)] = u
		}
	}
	return out, nil
}

// generate runs the shared question generator for a level and session.
func generate(c models.AngkaLevelContent, objectIDs []uuid.UUID, seed uint32) []angkagen.Question {
	ids := make([]string, len(objectIDs))
	for i, id := range objectIDs {
		ids[i] = id.String()
	}
	return angkagen.Generate(angkagen.Params{
		Min: c.Range.Min, Max: c.Range.Max, Count: c.QuestionCount,
		ObjectIDs: ids, Layout: c.Layout, Seed: seed,
	})
}

// starsFor turns first-try correct answers into 1–3 stars.
func starsFor(firstCorrect int, s models.AngkaStars) int {
	switch {
	case firstCorrect >= s.Three:
		return 3
	case firstCorrect >= s.Two:
		return 2
	default:
		return 1
	}
}
