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
)

func TestCategoryService_GetCategories_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewCategoryService(gormDB)

	id1 := uuid.New()
	id2 := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "name", "image_url", "created_at", "updated_at", "is_deleted",
	}).
		AddRow(id1, "Fabel", "https://img/fabel.png", now, now, false).
		AddRow(id2, "Legenda", "https://img/legenda.png", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "categories" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnRows(rows)

	result, err := svc.GetCategories()

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "Fabel", result[0].Name)
	assert.Equal(t, "Legenda", result[1].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_GetCategories_Empty(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewCategoryService(gormDB)

	rows := sqlmock.NewRows([]string{
		"id", "name", "image_url", "created_at", "updated_at", "is_deleted",
	})

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "categories" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnRows(rows)

	result, err := svc.GetCategories()

	require.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCategoryService_GetCategories_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewCategoryService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "categories" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnError(sql.ErrConnDone)

	result, err := svc.GetCategories()

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}
