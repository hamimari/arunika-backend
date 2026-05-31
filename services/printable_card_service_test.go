package services

import (
	"arunika_backend/models"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Printable Card (models.FindAllCards) ─────────────────────────────────────
// These tests exercise the DB query used by the printable card PDF endpoint.

// Scenario: valid category_id returns list of AR cards
func TestFindAllCards_ValidCategory_ReturnsCards(t *testing.T) {
	gormDB, mock := setupMockDB(t)

	catID := uuid.New()
	cardID := uuid.NewString()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code",
		"category", "sub_category", "image_url", "emoji", "bg_color",
		"is_unlocked", "description", "printable_img", "category_id", "sub_category_id",
		"created_at", "expires_at",
	}).AddRow(
		cardID, "animal", "Kuda", "https://cdn/kuda.glb", "", "K001",
		"", "", "https://cdn/kuda.png", "🐴", "#FFF3E0",
		true, "", "", &catID, nil,
		now, nil,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE category_id`)).
		WillReturnRows(rows)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	cards, err := models.FindAllCards(gormDB, catID.String(), "")
	require.NoError(t, err)
	assert.Len(t, cards, 1)
	assert.Equal(t, "Kuda", cards[0].Title)
}

// Scenario: category with no cards returns empty slice (not an error)
func TestFindAllCards_EmptyCategory_ReturnsEmptySlice(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	_ = mock

	catID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE category_id`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title"}))

	cards, err := models.FindAllCards(gormDB, catID.String(), "")
	require.NoError(t, err)
	assert.Empty(t, cards, "should return empty slice, not error")
}

// Scenario: unknown category_id returns empty result (no error — GORM Find does not error on empty)
func TestFindAllCards_UnknownCategory_ReturnsEmptyNotError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	_ = mock

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE category_id`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title"}))

	cards, err := models.FindAllCards(gormDB, uuid.NewString(), "")
	require.NoError(t, err)
	assert.Empty(t, cards)
}
