package services

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestArService_GetByID_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewArService(gormDB)

	id := "ar-card-1"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code", "created_at", "expires_at",
	}).AddRow(id, "model", "Dragon", "https://cdn/dragon.glb", "", "DRG", now, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 ORDER BY "ar_cards"."id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(rows)

	result, err := svc.GetByID(id)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, id, result.ID)
	assert.Equal(t, "Dragon", result.Title)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestArService_GetByID_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewArService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 ORDER BY "ar_cards"."id" LIMIT $2`)).
		WithArgs("missing-id", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	result, err := svc.GetByID("missing-id")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestArService_GetByID_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewArService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 ORDER BY "ar_cards"."id" LIMIT $2`)).
		WithArgs("any-id", 1).
		WillReturnError(sql.ErrConnDone)

	result, err := svc.GetByID("any-id")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}
