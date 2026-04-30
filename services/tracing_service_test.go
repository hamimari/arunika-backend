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

func tracingItemColumns() []string {
	return []string{"id", "type", "label", "guide_path_json", "difficulty", "created_at"}
}

// ─── GetItems ────────────────────────────────────────────────────────────────

func TestTracingGetItems_NoFilter(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewTracingService(db)

	id := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows(tracingItemColumns()).
		AddRow(id, "alphabet", "A", `[]`, 1, now)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "tracing_items" ORDER BY label ASC`)).
		WillReturnRows(rows)

	items, err := svc.GetItems("")
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "A", items[0].Label)
}

func TestTracingGetItems_WithTypeFilter(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewTracingService(db)

	rows := sqlmock.NewRows(tracingItemColumns())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "tracing_items" WHERE type = $1 ORDER BY label ASC`)).
		WithArgs("alphabet").
		WillReturnRows(rows)

	items, err := svc.GetItems("alphabet")
	require.NoError(t, err)
	assert.Empty(t, items)
}

// ─── SaveProgress ────────────────────────────────────────────────────────────

func TestTracingSaveProgress_Pass_AwardsBadge(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewTracingService(db)

	userID := uuid.New()
	childID := uuid.New()
	itemID := uuid.New()

	// Outer transaction (s.db.Transaction)
	mock.ExpectBegin()

	// INSERT tracing_progress — GORM columns: user_id, child_id, item_id, score, passed, created_at
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "tracing_progress"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	// Badge check: COUNT passed tracing progress
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "tracing_progress" WHERE user_id = $1 AND passed = true`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Load badges for feature = tracing (returns empty → no award)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "badges" WHERE feature = $1`)).
		WithArgs("tracing").
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature", "level", "threshold"}))

	// All-rounder check: tracing beginner count = 0 → short-circuit
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*)`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectCommit()

	req := SaveTracingProgressRequest{
		ItemID:  itemID,
		Score:   80,
		Passed:  true,
		ChildID: childID,
	}
	progress, err := svc.SaveProgress(userID, req)
	require.NoError(t, err)
	assert.Equal(t, 80, progress.Score)
	assert.True(t, progress.Passed)
}

func TestTracingSaveProgress_Fail_NoBadgeCheck(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewTracingService(db)

	userID := uuid.New()
	childID := uuid.New()
	itemID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "tracing_progress"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	req := SaveTracingProgressRequest{
		ItemID:  itemID,
		Score:   40,
		Passed:  false,
		ChildID: childID,
	}
	progress, err := svc.SaveProgress(userID, req)
	require.NoError(t, err)
	assert.False(t, progress.Passed)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GuidePathPoints ─────────────────────────────────────────────────────────

func TestGuidePathPoints_Valid(t *testing.T) {
	pts, err := GuidePathPoints(`[{"x":0,"y":0},{"x":1,"y":1}]`)
	require.NoError(t, err)
	assert.Len(t, pts, 2)
}

func TestGuidePathPoints_Invalid(t *testing.T) {
	_, err := GuidePathPoints(`not json`)
	assert.Error(t, err)
}
