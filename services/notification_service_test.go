package services

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── RegisterToken ────────────────────────────────────────────────────────────

func TestNotificationRegisterToken(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewNotificationService(db)

	userID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "fcm_tokens"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	err := svc.RegisterToken(userID, "device-token-abc")
	require.NoError(t, err)
}

// ─── GetNotifications ────────────────────────────────────────────────────────

func TestNotificationGetNotifications(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewNotificationService(db)
	userID := uuid.New()
	now := time.Now()

	cols := []string{"id", "user_id", "title", "body", "type", "is_read", "created_at"}
	rows := sqlmock.NewRows(cols).
		AddRow(uuid.New(), userID, "Test", "Body", "badge", false, now)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "notifications" WHERE user_id = $1 ORDER BY created_at DESC`)).
		WithArgs(userID).
		WillReturnRows(rows)

	notifs, err := svc.GetNotifications(userID)
	require.NoError(t, err)
	assert.Len(t, notifs, 1)
}

// ─── MarkRead ─────────────────────────────────────────────────────────────────

func TestNotificationMarkRead_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewNotificationService(db)

	userID := uuid.New()
	notifID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "notifications" SET "is_read"=$1 WHERE id = $2 AND user_id = $3`)).
		WithArgs(true, notifID, userID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.MarkRead(userID, notifID)
	require.NoError(t, err)
}

func TestNotificationMarkRead_NotFound(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewNotificationService(db)

	userID := uuid.New()
	notifID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "notifications" SET "is_read"=$1 WHERE id = $2 AND user_id = $3`)).
		WithArgs(true, notifID, userID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.MarkRead(userID, notifID)
	assert.Error(t, err)
}
