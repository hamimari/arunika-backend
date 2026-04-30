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
	"gorm.io/gorm"
)

func TestUserService_GetUserByID_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	userID := uuid.New()
	childID := uuid.New()
	now := time.Now()

	parentRows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(userID, "Alice", "081", "alice@example.com", "hash",
		"Jl. Test", "Jakarta", now, now, false)

	// GORM Preload("Children") executes a second query
	childRows := sqlmock.NewRows([]string{
		"id", "parent_id", "name", "gender", "date_of_birth", "created_at", "updated_at", "is_deleted",
	}).AddRow(childID, userID.String(), "Bob", "male", now, now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(parentRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "children" WHERE "children"."parent_id" = $1`)).
		WithArgs(userID.String()).
		WillReturnRows(childRows)

	result, _, err := svc.GetUserByID(userID.String())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Alice", result.Name)
	assert.Len(t, result.Children, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetUserByID_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("missing-id", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	result, _, err := svc.GetUserByID("missing-id")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetUserByID_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("any-id", 1).
		WillReturnError(sql.ErrConnDone)

	result, _, err := svc.GetUserByID("any-id")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}
