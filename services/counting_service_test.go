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

func countingQuestionColumns() []string {
	return []string{"id", "level", "question_json", "answer", "created_at"}
}

// ─── GetQuestions ────────────────────────────────────────────────────────────

func TestCountingGetQuestions_NoFilter(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewCountingService(db)

	id := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows(countingQuestionColumns()).
		AddRow(id, "easy", `{}`, 3, now)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "counting_questions"`)).
		WillReturnRows(rows)

	questions, err := svc.GetQuestions("")
	require.NoError(t, err)
	assert.Len(t, questions, 1)
}

func TestCountingGetQuestions_WithLevel(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewCountingService(db)

	rows := sqlmock.NewRows(countingQuestionColumns())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "counting_questions" WHERE level = $1`)).
		WithArgs("hard").
		WillReturnRows(rows)

	questions, err := svc.GetQuestions("hard")
	require.NoError(t, err)
	assert.Empty(t, questions)
}

// ─── SaveProgress ────────────────────────────────────────────────────────────

func TestCountingSaveProgress_Correct_AwardsBadge(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewCountingService(db)

	userID := uuid.New()
	childID := uuid.New()
	questionID := uuid.New()

	mock.ExpectBegin()

	// INSERT counting_progress
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "counting_progress"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	// Badge check: COUNT correct counting answers
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "counting_progress" WHERE user_id = $1 AND is_correct = true`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Load badges for counting (empty → no award)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "badges" WHERE feature = $1`)).
		WithArgs("counting").
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature", "level", "threshold"}))

	// All-rounder check
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*)`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectCommit()

	req := SaveCountingProgressRequest{
		QuestionID: questionID,
		IsCorrect:  true,
		ChildID:    childID,
	}
	progress, err := svc.SaveProgress(userID, req)
	require.NoError(t, err)
	assert.True(t, progress.IsCorrect)
}

func TestCountingSaveProgress_Wrong_NoBadgeCheck(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewCountingService(db)

	userID := uuid.New()
	childID := uuid.New()
	questionID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "counting_progress"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	req := SaveCountingProgressRequest{
		QuestionID: questionID,
		IsCorrect:  false,
		ChildID:    childID,
	}
	progress, err := svc.SaveProgress(userID, req)
	require.NoError(t, err)
	assert.False(t, progress.IsCorrect)
	assert.NoError(t, mock.ExpectationsWereMet())
}
