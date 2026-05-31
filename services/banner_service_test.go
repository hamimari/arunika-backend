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

func TestBannerService_GetActiveBanners_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewBannerService(gormDB)

	id1 := uuid.New()
	id2 := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "title", "image_url", "type", "is_active", "sort_order",
		"cta_url", "emoji", "fact", "created_at", "updated_at", "is_deleted",
	}).
		AddRow(id1, "Promo Hari Ini", "https://cdn/promo.png", "promo", true, 0,
			nil, nil, nil, now, now, false).
		AddRow(id2, "Daily Animal", "https://cdn/animal.png", "daily_animal", true, 1,
			nil, "🐄", "Sapi punya empat lambung.", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "banners" WHERE is_active = $1 AND is_deleted = $2 ORDER BY sort_order ASC`,
	)).WithArgs(true, false).WillReturnRows(rows)

	banners, err := svc.GetActiveBanners()

	require.NoError(t, err)
	assert.Len(t, banners, 2)
	assert.Equal(t, "Promo Hari Ini", banners[0].Title)
	assert.Equal(t, "daily_animal", banners[1].Type)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBannerService_GetActiveBanners_Empty(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewBannerService(gormDB)

	rows := sqlmock.NewRows([]string{
		"id", "title", "image_url", "type", "is_active", "sort_order",
		"cta_url", "emoji", "fact", "created_at", "updated_at", "is_deleted",
	})

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "banners" WHERE is_active = $1 AND is_deleted = $2 ORDER BY sort_order ASC`,
	)).WithArgs(true, false).WillReturnRows(rows)

	banners, err := svc.GetActiveBanners()

	require.NoError(t, err)
	assert.Empty(t, banners)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBannerService_GetActiveBanners_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewBannerService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "banners" WHERE is_active = $1 AND is_deleted = $2 ORDER BY sort_order ASC`,
	)).WithArgs(true, false).WillReturnError(gorm.ErrInvalidDB)

	banners, err := svc.GetActiveBanners()

	assert.Error(t, err)
	assert.Nil(t, banners)
	assert.NoError(t, mock.ExpectationsWereMet())
}
