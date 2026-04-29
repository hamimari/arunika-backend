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
)

// setupMockDB creates an in-memory mock SQL database wrapped by GORM.
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	t.Cleanup(func() { db.Close() })

	dialector := postgres.New(postgres.Config{
		Conn:       db,
		DriverName: "postgres",
	})
	gormDB, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	return gormDB, mock
}

// ─── GetFairyTales tests ──────────────────────────────────────────────────────

func dongengColumns() []string {
	return []string{
		"id", "title", "age_start", "age_end", "image_url", "audio_url",
		"is_free", "category_id", "duration", "created_at", "updated_at", "is_deleted",
	}
}

func TestGetFairyTales_ReturnsAllActive(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewDongengService(gormDB)

	id1 := uuid.New()
	id2 := uuid.New()
	now := time.Now()

	// COUNT query
	countRows := sqlmock.NewRows([]string{"count"}).AddRow(2)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnRows(countRows)

	// SELECT query
	rows := sqlmock.NewRows(dongengColumns()).
		AddRow(id1, "Poor Pluto", 6, 9, "http://img/pluto.png", "", false, nil, int64(300), now, now, false).
		AddRow(id2, "Hansel & Gretel", 9, 12, "http://img/hansel.png", "", true, nil, int64(600), now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE is_deleted = $1 LIMIT $2`)).
		WithArgs(false, 10).
		WillReturnRows(rows)

	result, err := svc.GetFairyTales("", 1, 10)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Items, 2)

	assert.Equal(t, id1, result.Items[0].ID)
	assert.Equal(t, "Poor Pluto", result.Items[0].Title)
	assert.Equal(t, "5 min", result.Items[0].Duration)

	assert.Equal(t, id2, result.Items[1].ID)
	assert.Equal(t, "Hansel & Gretel", result.Items[1].Title)
	assert.Equal(t, "10 min", result.Items[1].Duration)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTales_EmptyDatabase(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewDongengService(gormDB)

	countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnRows(countRows)

	rows := sqlmock.NewRows(dongengColumns())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE is_deleted = $1 LIMIT $2`)).
		WithArgs(false, 10).
		WillReturnRows(rows)

	result, err := svc.GetFairyTales("", 1, 10)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Items)
	assert.Equal(t, int64(0), result.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTales_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewDongengService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnError(sql.ErrConnDone)

	result, err := svc.GetFairyTales("", 1, 10)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GetFairyTaleByID tests ───────────────────────────────────────────────────

func TestGetFairyTaleByID_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewDongengService(gormDB)

	dongengID := uuid.New()
	pageID := uuid.New()
	now := time.Now()

	dongengRows := sqlmock.NewRows(dongengColumns()).
		AddRow(dongengID, "Poor Pluto", 6, 9, "http://img/pluto.png", "", false, nil, int64(300), now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE id = $1 AND is_deleted = $2 ORDER BY "dongengs"."id" LIMIT $3`)).
		WithArgs(dongengID.String(), false, 1).
		WillReturnRows(dongengRows)

	pageRows := sqlmock.NewRows([]string{
		"id", "dongeng_id", "page_number", "image_url", "text", "audio_url", "created_at", "updated_at", "is_deleted",
	}).AddRow(pageID, dongengID, 1, "http://img/page1.png", "Once upon a time...", "", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongeng_pages" WHERE is_deleted = $1 AND "dongeng_pages"."dongeng_id" = $2 ORDER BY page_number ASC`)).
		WithArgs(false, dongengID).
		WillReturnRows(pageRows)

	result, err := svc.GetFairyTaleByID(dongengID.String())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, dongengID, result.ID)
	assert.Equal(t, "Poor Pluto", result.Title)
	assert.Equal(t, "5 min", result.Duration)
	require.Len(t, result.Pages, 1)
	assert.Equal(t, 1, result.Pages[0].PageNumber)
	assert.Equal(t, "Once upon a time...", result.Pages[0].Text)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTaleByID_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewDongengService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE id = $1 AND is_deleted = $2 ORDER BY "dongengs"."id" LIMIT $3`)).
		WithArgs("non-existent-id", false, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	result, err := svc.GetFairyTaleByID("non-existent-id")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}
