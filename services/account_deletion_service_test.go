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

func TestAccountDeletionService_DeleteAccount_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAccountDeletionService(gormDB)

	userID := uuid.New()
	childID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 AND is_deleted = false`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows(parentCols()).
			AddRow(userID, now, now, false, "Jane", "0812", "jane@example.com", "hash", "Jl. Mawar", "Jakarta"))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "id" FROM "children" WHERE parent_id = $1`)).
		WithArgs(userID.String()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(childID))

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "growth_records" WHERE child_id IN ($1)`)).
		WithArgs(childID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "children" WHERE parent_id = $1`)).
		WithArgs(userID.String()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	for _, table := range []string{
		"user_badges", "tracing_progress", "counting_progress", "dongeng_play_history",
		"user_entitlements", "user_subscriptions", "notifications", "fcm_tokens",
		"user_sessions", "refresh_tokens",
	} {
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "` + table + `" WHERE user_id = $1`)).
			WithArgs(userID).
			WillReturnResult(sqlmock.NewResult(0, 0))
	}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "parents" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	err := svc.DeleteAccount(userID)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountDeletionService_DeleteAccount_AlreadyDeleted(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAccountDeletionService(gormDB)

	userID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 AND is_deleted = false`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows(parentCols()))
	mock.ExpectRollback()

	err := svc.DeleteAccount(userID)

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}
