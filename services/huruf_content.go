package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"arunika_backend/hurufpath"
	"arunika_backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Publish validation limits (design D5).
const (
	maxWordRunes     = 20
	maxFeedbackRunes = 80
	maxHintRunes     = 160
)

// FieldError names one invalid draft field by its JSON path.
type FieldError struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

// ValidationFailedError lists every reason a draft can't be published.
type ValidationFailedError struct{ Fields []FieldError }

func (e *ValidationFailedError) Error() string {
	return fmt.Sprintf("validation failed (%d fields)", len(e.Fields))
}

func parseContent(raw []byte) (models.LetterContent, error) {
	var c models.LetterContent
	err := json.Unmarshal(raw, &c)
	return c, err
}

// canonical re-encodes content so drafts and versions compare by value,
// regardless of key order or whitespace in the stored JSON.
func canonical(raw []byte) string {
	c, err := parseContent(raw)
	if err != nil {
		return string(raw)
	}
	b, _ := json.Marshal(c)
	return string(b)
}

// assetIDs returns the content's referenced asset ids.
func assetIDs(c models.LetterContent) []uuid.UUID {
	var ids []uuid.UUID
	for _, id := range []*uuid.UUID{c.Kenali.ImageAssetID, c.Dengar.LetterAudioID, c.Dengar.WordAudioID} {
		if id != nil {
			ids = append(ids, *id)
		}
	}
	return ids
}

// validateContent applies the publish rules. assets holds the referenced
// assets that exist.
func validateContent(c models.LetterContent, upper string, assets map[uuid.UUID]AssetView) []FieldError {
	var errs []FieldError
	add := func(path, code string) { errs = append(errs, FieldError{Path: path, Code: code}) }

	word := []rune(c.Kenali.Word)
	switch {
	case strings.TrimSpace(c.Kenali.Word) == "":
		add("kenali.word", "REQUIRED")
	case len(word) > maxWordRunes:
		add("kenali.word", "TOO_LONG")
	}
	if len(c.Kenali.Highlight) == 0 {
		if len(word) > 0 {
			add("kenali.highlight", "REQUIRED")
		}
	} else {
		for _, i := range c.Kenali.Highlight {
			if i < 0 || i >= len(word) || !strings.EqualFold(string(word[i]), upper) {
				add("kenali.highlight", "INVALID")
				break
			}
		}
	}

	checkMedia := func(path string, id *uuid.UUID, external, urlPath, kind string) {
		if id == nil {
			switch {
			case external == "":
				add(path, "REQUIRED")
			case !validMediaURL(external):
				add(urlPath, "INVALID_URL")
			}
			return
		}
		a, ok := assets[*id]
		switch {
		case !ok:
			add(path, "NOT_FOUND")
		case a.Kind != kind:
			add(path, "WRONG_KIND")
		}
	}
	checkMedia("kenali.image_asset_id", c.Kenali.ImageAssetID, c.Kenali.ImageURL, "kenali.image_url", models.AssetKindImage)
	checkMedia("dengar.letter_audio_id", c.Dengar.LetterAudioID, c.Dengar.LetterAudioURL, "dengar.letter_audio_url", models.AssetKindAudio)
	checkMedia("dengar.word_audio_id", c.Dengar.WordAudioID, c.Dengar.WordAudioURL, "dengar.word_audio_url", models.AssetKindAudio)

	checkStrokes := func(name string, strokes []models.LetterStroke, required bool) {
		path := "tebalkan." + name
		if len(strokes) == 0 {
			if required {
				add(path, "REQUIRED")
			}
			return
		}
		orders := make([]int, len(strokes))
		for i, s := range strokes {
			orders[i] = s.Order
			if _, err := hurufpath.Parse(s.Path); err != nil {
				add(fmt.Sprintf("%s[%d].path", path, i), "INVALID_PATH")
			}
		}
		sort.Ints(orders)
		for i, o := range orders {
			if o != i+1 {
				add(path, "ORDER_INVALID")
				break
			}
		}
	}
	checkStrokes("upper", c.Tebalkan.Upper, true)
	checkStrokes("lower", c.Tebalkan.Lower, c.Tebalkan.LowerRequired)

	checkText := func(path, v string, max int) {
		switch {
		case strings.TrimSpace(v) == "":
			add(path, "REQUIRED")
		case utf8.RuneCountInString(v) > max:
			add(path, "TOO_LONG")
		}
	}
	checkText("feedback.success", c.Feedback.Success, maxFeedbackRunes)
	checkText("feedback.retry", c.Feedback.Retry, maxFeedbackRunes)
	checkText("feedback.hint", c.Feedback.Hint, maxHintRunes)
	return errs
}

// LetterView is letter content as the app (and the backoffice preview)
// renders it: asset ids resolved to URLs.
type LetterView struct {
	ID       uuid.UUID             `json:"id"`
	Version  int                   `json:"version"`
	Upper    string                `json:"upper"`
	Lower    string                `json:"lower"`
	Kenali   LetterKenaliView      `json:"kenali"`
	Dengar   LetterDengarView      `json:"dengar"`
	Tebalkan models.LetterTebalkan `json:"tebalkan"`
	Feedback models.LetterFeedback `json:"feedback"`
}

type LetterKenaliView struct {
	Word      string `json:"word"`
	Highlight []int  `json:"highlight"`
	ImageURL  string `json:"image_url"`
}

type LetterDengarView struct {
	LetterAudioURL string `json:"letter_audio_url"`
	WordAudioURL   string `json:"word_audio_url"`
}

// mediaURL is a media slot's URL: the uploaded asset's, or else the
// external URL entered for it.
func mediaURL(assets map[uuid.UUID]AssetView, id *uuid.UUID, external string) string {
	if id == nil {
		return external
	}
	return assets[*id].URL
}

// normalizeMedia trims the external URLs and drops any that an uploaded
// asset overrides, so a draft never stores both for one slot.
func normalizeMedia(c *models.LetterContent) {
	pick := func(id *uuid.UUID, u *string) {
		*u = strings.TrimSpace(*u)
		if id != nil {
			*u = ""
		}
	}
	pick(c.Kenali.ImageAssetID, &c.Kenali.ImageURL)
	pick(c.Dengar.LetterAudioID, &c.Dengar.LetterAudioURL)
	pick(c.Dengar.WordAudioID, &c.Dengar.WordAudioURL)
}

// validMediaURL accepts absolute https URLs only: the app refuses cleartext.
func validMediaURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// audioCount is how many of a draft's two audio slots are filled.
func audioCount(d models.LetterContent) int {
	n := 0
	if d.Dengar.LetterAudioID != nil || d.Dengar.LetterAudioURL != "" {
		n++
	}
	if d.Dengar.WordAudioID != nil || d.Dengar.WordAudioURL != "" {
		n++
	}
	return n
}

// buildLetterView resolves c's assets for presentation.
func buildLetterView(tx *gorm.DB, as *AssetService, id uuid.UUID, version int, c models.LetterContent) (*LetterView, error) {
	assets, err := as.Find(tx, assetIDs(c))
	if err != nil {
		return nil, err
	}
	sortStrokes(c.Tebalkan.Upper)
	sortStrokes(c.Tebalkan.Lower)
	if c.Tebalkan.Upper == nil {
		c.Tebalkan.Upper = []models.LetterStroke{}
	}
	if c.Tebalkan.Lower == nil {
		c.Tebalkan.Lower = []models.LetterStroke{}
	}
	if c.Kenali.Highlight == nil {
		c.Kenali.Highlight = []int{}
	}
	if c.Tebalkan.Grid == 0 {
		c.Tebalkan.Grid = hurufpath.Grid
	}
	return &LetterView{
		ID:      id,
		Version: version,
		Upper:   c.Upper,
		Lower:   c.Lower,
		Kenali: LetterKenaliView{
			Word:      c.Kenali.Word,
			Highlight: c.Kenali.Highlight,
			ImageURL:  mediaURL(assets, c.Kenali.ImageAssetID, c.Kenali.ImageURL),
		},
		Dengar: LetterDengarView{
			LetterAudioURL: mediaURL(assets, c.Dengar.LetterAudioID, c.Dengar.LetterAudioURL),
			WordAudioURL:   mediaURL(assets, c.Dengar.WordAudioID, c.Dengar.WordAudioURL),
		},
		Tebalkan: c.Tebalkan,
		Feedback: c.Feedback,
	}, nil
}

func sortStrokes(s []models.LetterStroke) {
	sort.SliceStable(s, func(i, j int) bool { return s[i].Order < s[j].Order })
}
