package services

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func premiumPackColumns() []string {
	return []string{
		"id", "name", "subtitle", "price_idr", "type",
		"badge_label", "is_best_value", "is_active", "sort_order",
		"created_at", "updated_at",
	}
}

// ─── GetAllPacks ───────────────────────────────────────────────────────────────

func TestPremiumPackService_GetAllPacks_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB)

	id1 := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow(id1, "Basic", "Akses konten dasar", 49000, "content",
			nil, false, true, 1, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" ORDER BY sort_order asc`,
	)).WillReturnRows(rows)

	packs, err := svc.GetAllPacks()

	require.NoError(t, err)
	assert.Len(t, packs, 1)
	assert.Equal(t, "Basic", packs[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPremiumPackService_GetAllPacks_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" ORDER BY sort_order asc`,
	)).WillReturnError(gorm.ErrInvalidDB)

	packs, err := svc.GetAllPacks()

	assert.Error(t, err)
	assert.Nil(t, packs)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GetActivePacks ────────────────────────────────────────────────────────────

func TestPremiumPackService_GetActivePacks_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB)

	id1 := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow(id1, "Premium", "Akses penuh", 99000, "subscription",
			nil, true, true, 0, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" WHERE is_active = true ORDER BY sort_order asc`,
	)).WillReturnRows(rows)

	packs, err := svc.GetActivePacks("")

	require.NoError(t, err)
	assert.Len(t, packs, 1)
	assert.Equal(t, "Premium", packs[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPremiumPackService_GetActivePacks_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" WHERE is_active = true ORDER BY sort_order asc`,
	)).WillReturnError(gorm.ErrInvalidDB)

	packs, err := svc.GetActivePacks("")

	assert.Error(t, err)
	assert.Nil(t, packs)
	assert.NoError(t, mock.ExpectationsWereMet())
}
