package services

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func featureFlagCols() []string {
	return []string{"key", "name", "description", "is_enabled", "updated_at"}
}

func TestFeatureFlagService_EnabledMap(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewFeatureFlagService(db)
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "app_feature_flags" ORDER BY key ASC`)).
		WillReturnRows(sqlmock.NewRows(featureFlagCols()).
			AddRow("printable_cards", "Kartu Printable", "", true, now).
			AddRow("qr_scan", "Scan QR", "", false, now))

	flags, err := svc.EnabledMap()
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"printable_cards": true, "qr_scan": false}, flags)
}

func TestFeatureFlagService_SetEnabled_Success(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewFeatureFlagService(db)
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "app_feature_flags" SET "is_enabled"=$1,"updated_at"=$2 WHERE key = $3`)).
		WithArgs(false, sqlmock.AnyArg(), "qr_scan").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "app_feature_flags" WHERE key = $1`)).
		WithArgs("qr_scan", 1).
		WillReturnRows(sqlmock.NewRows(featureFlagCols()).AddRow("qr_scan", "Scan QR", "", false, now))

	flag, err := svc.SetEnabled("qr_scan", false)
	require.NoError(t, err)
	assert.False(t, flag.IsEnabled)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFeatureFlagService_SetEnabled_UnknownKey(t *testing.T) {
	db, mock := setupMockDB(t)
	svc := NewFeatureFlagService(db)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "app_feature_flags"`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	_, err := svc.SetEnabled("nope", true)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
