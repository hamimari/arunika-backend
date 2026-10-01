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

func badgeColumns() []string {
	return []string{"id", "feature", "level", "threshold"}
}

// ─── CheckAndAward ────────────────────────────────────────────────────────────

func TestBadgeCheckAndAward_BelowThreshold_NoInsert(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewBadgeService(db)
	userID := uuid.New()

	// COUNT tracing progress
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "tracing_progress" WHERE user_id = $1 AND passed = true`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Badges for tracing: beginner at 5 → 0 < 5 → no insert
	badgeID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "badges" WHERE feature = $1`)).
		WithArgs("tracing").
		WillReturnRows(sqlmock.NewRows(badgeColumns()).AddRow(badgeID, "tracing", "beginner", 5))

	// All-rounder check (no beginner tracing badge)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*)`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err := svc.CheckAndAward(userID, "tracing")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBadgeCheckAndAward_AboveThreshold_Inserts(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewBadgeService(db)
	userID := uuid.New()

	// COUNT: 5 passed
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "tracing_progress" WHERE user_id = $1 AND passed = true`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	badgeID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "badges" WHERE feature = $1`)).
		WithArgs("tracing").
		WillReturnRows(sqlmock.NewRows(badgeColumns()).AddRow(badgeID, "tracing", "beginner", 5))

	// GORM Create wraps in transaction for ON CONFLICT INSERT
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_badges"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	// All-rounder: tracing beginner exists → check counting
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*)`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*)`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err := svc.CheckAndAward(userID, "tracing")
	require.NoError(t, err)
}

// ─── Idempotent award ─────────────────────────────────────────────────────────

func TestBadgeCheckAndAward_Idempotent(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewBadgeService(db)
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "tracing_progress"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	badgeID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "badges"`)).
		WillReturnRows(sqlmock.NewRows(badgeColumns()).AddRow(badgeID, "tracing", "beginner", 5))

	// ON CONFLICT DO NOTHING — no rows inserted (idempotent)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_badges"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*)`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	err := svc.CheckAndAward(userID, "tracing")
	require.NoError(t, err)
}

// ─── GetUserBadges ────────────────────────────────────────────────────────────

func TestGetUserBadges_ReturnsMixedEarnedStatus(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewBadgeService(db)
	userID := uuid.New()

	badgeID1 := uuid.New()
	badgeID2 := uuid.New()
	now := time.Now()
	_ = now

	// All badges
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "badges"`)).
		WillReturnRows(sqlmock.NewRows(badgeColumns()).
			AddRow(badgeID1, "tracing", "beginner", 5).
			AddRow(badgeID2, "counting", "beginner", 5))

	// Earned badge IDs
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "badge_id" FROM "user_badges" WHERE user_id = $1`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"badge_id"}).AddRow(badgeID1))

	// Tracing progress count
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "tracing_progress" WHERE user_id = $1 AND passed = true`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	// Counting progress count
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "counting_progress" WHERE user_id = $1 AND is_correct = true`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	badges, err := svc.GetUserBadges(userID)
	require.NoError(t, err)
	require.Len(t, badges, 2)
	assert.True(t, badges[0].Earned)
	assert.False(t, badges[1].Earned)
	assert.Equal(t, int64(7), badges[0].Progress)
	assert.Equal(t, int64(2), badges[1].Progress)
}
