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

func growthColumns() []string {
	return []string{"id", "child_id", "recorded_at", "weight_kg", "height_cm", "created_at"}
}

// ─── Save ─────────────────────────────────────────────────────────────────────

func TestGrowthSave(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)

	childID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "growth_records"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	req := SaveGrowthRequest{
		ChildID:    childID,
		WeightKg:   12.5,
		HeightCm:   90.0,
		RecordedAt: now,
	}
	record, err := svc.Save(req)
	require.NoError(t, err)
	assert.Equal(t, childID, record.ChildID)
	assert.Equal(t, 12.5, record.WeightKg)
}

func TestGrowthSave_DefaultsRecordedAt(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)

	childID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "growth_records"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	req := SaveGrowthRequest{
		ChildID:  childID,
		WeightKg: 10.0,
		HeightCm: 80.0,
		// RecordedAt zero — should default to now
	}
	record, err := svc.Save(req)
	require.NoError(t, err)
	assert.False(t, record.RecordedAt.IsZero())
}

// ─── GetHistory ──────────────────────────────────────────────────────────────

func TestGrowthGetHistory(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)

	childID := uuid.New()
	now := time.Now()

	t1 := now.AddDate(0, 0, -7)
	t2 := now

	rows := sqlmock.NewRows(growthColumns()).
		AddRow(uuid.New(), childID, t1, 10.0, 80.0, now).
		AddRow(uuid.New(), childID, t2, 10.5, 81.0, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "growth_records" WHERE child_id = $1 ORDER BY recorded_at ASC`)).
		WithArgs(childID).
		WillReturnRows(rows)

	records, err := svc.GetHistory(childID)
	require.NoError(t, err)
	assert.Len(t, records, 2)
	assert.True(t, records[0].RecordedAt.Before(records[1].RecordedAt))
}
