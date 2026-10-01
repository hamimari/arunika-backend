package services

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"arunika_backend/models"
)

func setupBannerDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	dialector := postgres.New(postgres.Config{Conn: db, DriverName: "postgres"})
	gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return gormDB, mock
}

func bannerCols() []string {
	return []string{"id", "title", "image_url", "link_url", "description", "is_active", "sort_order", "hidden", "created_at", "updated_at", "is_deleted"}
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestBannerService_List_Success(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "banners" WHERE is_deleted = false`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery(`SELECT \* FROM "banners" WHERE is_deleted = false`).
		WillReturnRows(sqlmock.NewRows(bannerCols()).
			AddRow(id, "Summer Sale", "https://img/summer.png", "", "", true, 1, false, now, now, false))

	items, total, err := svc.List("", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 1)
}

func TestBannerService_List_WithSearch(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "banners" WHERE is_deleted = false AND title ILIKE $1`)).
		WithArgs("%sale%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(`SELECT \* FROM "banners" WHERE is_deleted = false AND title ILIKE`).
		WillReturnRows(sqlmock.NewRows(bannerCols()))

	items, total, err := svc.List("sale", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, items)
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestBannerService_Get_NotFound(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	mock.ExpectQuery(`SELECT \* FROM "banners" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Get("non-existent")
	assert.Error(t, err)
}

func TestBannerService_Get_Success(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "banners" WHERE id = $1 AND is_deleted = false ORDER BY "banners"."id" LIMIT $2`)).
		WithArgs(id.String(), 1).
		WillReturnRows(sqlmock.NewRows(bannerCols()).
			AddRow(id, "Holiday", "https://img/hol.png", "", "", true, 0, false, now, now, false))

	banner, err := svc.Get(id.String())
	require.NoError(t, err)
	assert.Equal(t, "Holiday", banner.Title)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestBannerService_Delete_Success(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	id := uuid.New().String()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "banners" SET "is_deleted"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Delete(id)
	assert.NoError(t, err)
}

// ─── ToggleVisibility ─────────────────────────────────────────────────────────

func TestBannerService_ToggleVisibility(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	id := uuid.New().String()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "banners" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ToggleVisibility(id, true)
	assert.NoError(t, err)
}

// ─── ToggleActive ─────────────────────────────────────────────────────────────

func TestBannerService_ToggleActive_Deactivate(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	id := uuid.New().String()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "banners" SET "is_active"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(false, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ToggleActive(id, false)
	assert.NoError(t, err)
}

// ─── GetActiveBanners ─────────────────────────────────────────────────────────

func TestBannerService_GetActiveBanners_Success(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT \* FROM "banners" WHERE is_deleted = false AND hidden = false AND is_active = true`).
		WillReturnRows(sqlmock.NewRows(bannerCols()).
			AddRow(id, "Active Banner", "https://img/active.png", "", "", true, 1, false, now, now, false))

	banners, err := svc.GetActiveBanners()
	require.NoError(t, err)
	assert.Len(t, banners, 1)
	assert.True(t, banners[0].IsActive)
}

func TestBannerService_GetActiveBanners_Empty(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	mock.ExpectQuery(`SELECT \* FROM "banners" WHERE is_deleted = false AND hidden = false AND is_active = true`).
		WillReturnRows(sqlmock.NewRows(bannerCols()))

	banners, err := svc.GetActiveBanners()
	require.NoError(t, err)
	assert.Empty(t, banners)
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestBannerService_Create_Success(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	input := models.Banner{
		Title:    "New Year Sale",
		ImageURL: "https://img/ny.png",
		IsActive: true,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "banners"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	result, err := svc.Create(input)
	require.NoError(t, err)
	assert.Equal(t, "New Year Sale", result.Title)
}

func TestBannerService_Create_DBError(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "banners"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := svc.Create(models.Banner{Title: "Fail"})
	assert.Error(t, err)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestBannerService_Update_NotFound(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	mock.ExpectQuery(`SELECT \* FROM "banners" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Update("missing", models.Banner{})
	assert.EqualError(t, err, "not found")
}

func TestBannerService_Update_Success(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "banners" WHERE id = $1 AND is_deleted = false ORDER BY "banners"."id" LIMIT $2`)).
		WithArgs(id.String(), 1).
		WillReturnRows(sqlmock.NewRows(bannerCols()).
			AddRow(id, "Old Title", "https://img/old.png", "", "", true, 0, false, now, now, false))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "banners" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.Update(id.String(), models.Banner{Title: "Updated Title"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestBannerService_Update_DBError(t *testing.T) {
	db, mock := setupBannerDB(t)
	svc := NewBannerService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "banners" WHERE id = $1 AND is_deleted = false ORDER BY "banners"."id" LIMIT $2`)).
		WithArgs(id.String(), 1).
		WillReturnRows(sqlmock.NewRows(bannerCols()).
			AddRow(id, "Title", "https://img/t.png", "", "", true, 0, false, now, now, false))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "banners" SET`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := svc.Update(id.String(), models.Banner{Title: "Fail"})
	assert.Error(t, err)
}
