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

func growthColumns() []string {
	return []string{"id", "child_id", "recorded_at", "weight_kg", "height_cm", "created_at"}
}

// ─── Save ─────────────────────────────────────────────────────────────────────

func TestGrowthSave(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)

	childID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "children" WHERE id = $1 AND parent_id = $2`)).
		WithArgs(childID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

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
	record, err := svc.Save(userID, req)
	require.NoError(t, err)
	assert.Equal(t, childID, record.ChildID)
	assert.Equal(t, 12.5, record.WeightKg)
}

func TestGrowthSave_DefaultsRecordedAt(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)

	childID := uuid.New()
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "children" WHERE id = $1 AND parent_id = $2`)).
		WithArgs(childID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

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
	record, err := svc.Save(userID, req)
	require.NoError(t, err)
	assert.False(t, record.RecordedAt.IsZero())
}

// ─── GetHistory ──────────────────────────────────────────────────────────────

func TestGrowthGetHistory(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)

	childID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "children" WHERE id = $1 AND parent_id = $2`)).
		WithArgs(childID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	t1 := now.AddDate(0, 0, -7)
	t2 := now

	rows := sqlmock.NewRows(growthColumns()).
		AddRow(uuid.New(), childID, t1, 10.0, 80.0, now).
		AddRow(uuid.New(), childID, t2, 10.5, 81.0, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "growth_records" WHERE child_id = $1 ORDER BY recorded_at ASC`)).
		WithArgs(childID).
		WillReturnRows(rows)

	records, err := svc.GetHistory(userID, childID)
	require.NoError(t, err)
	assert.Len(t, records, 2)
	assert.True(t, records[0].RecordedAt.Before(records[1].RecordedAt))
}

// ─── Ownership ───────────────────────────────────────────────────────────────

func expectNotOwner(mock sqlmock.Sqlmock, childID, userID uuid.UUID) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "children" WHERE id = $1 AND parent_id = $2`)).
		WithArgs(childID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
}

func TestGrowthSave_ChildOfAnotherUser_IsRejectedWithoutInserting(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)
	childID, userID := uuid.New(), uuid.New()
	expectNotOwner(mock, childID, userID)

	record, err := svc.Save(userID, SaveGrowthRequest{ChildID: childID, WeightKg: 10, HeightCm: 80})

	assert.ErrorIs(t, err, ErrChildNotFound)
	assert.Nil(t, record)
	assert.NoError(t, mock.ExpectationsWereMet(), "no INSERT may follow a failed ownership check")
}

func TestGrowthGetHistory_ChildOfAnotherUser_IsRejectedWithoutReading(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)
	childID, userID := uuid.New(), uuid.New()
	expectNotOwner(mock, childID, userID)

	records, err := svc.GetHistory(userID, childID)

	assert.ErrorIs(t, err, ErrChildNotFound)
	assert.Empty(t, records)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGrowthUpdate_RecordOfAnotherUsersChild_IsRejectedWithoutUpdating(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)
	recordID, childID, userID := uuid.New(), uuid.New(), uuid.New()

	mock.ExpectQuery(`SELECT \* FROM "growth_records" WHERE id = \$1`).
		WillReturnRows(sqlmock.NewRows(growthColumns()).
			AddRow(recordID, childID, time.Now(), 10.0, 80.0, time.Now()))
	expectNotOwner(mock, childID, userID)

	record, err := svc.Update(userID, recordID, UpdateGrowthRequest{WeightKg: 1, HeightCm: 1})

	assert.ErrorIs(t, err, ErrChildNotFound)
	assert.Nil(t, record)
	assert.NoError(t, mock.ExpectationsWereMet(), "no UPDATE may follow a failed ownership check")
}

func TestGrowthUpdate_UnknownRecord_LooksLikeAnotherUsersRecord(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewGrowthService(db)

	mock.ExpectQuery(`SELECT \* FROM "growth_records" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Update(uuid.New(), uuid.New(), UpdateGrowthRequest{WeightKg: 1, HeightCm: 1})

	assert.ErrorIs(t, err, ErrChildNotFound, "a missing record and a foreign one must be indistinguishable")
}
