package services

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAnalyticsDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	dialector := postgres.New(postgres.Config{Conn: db, DriverName: "postgres"})
	gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return gormDB, mock
}

// ─── GetSubscriptionStats ─────────────────────────────────────────────────────

func TestAdminAnalyticsService_GetSubscriptionStats_Success(t *testing.T) {
	db, mock := setupAnalyticsDB(t)
	rdb, rMock := redismock.NewClientMock()

	// Redis miss
	rMock.ExpectGet("analytics:subscription-stats").RedisNil()
	// Redis set (ignore error)
	rMock.ExpectSet("analytics:subscription-stats", sqlmock.AnyArg(), 0).SetVal("OK")

	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"total", "premium", "free"}).AddRow(10, 3, 7))

	svc := NewAdminAnalyticsService(db, rdb)
	stats, err := svc.GetSubscriptionStats()
	require.NoError(t, err)
	assert.Equal(t, int64(10), stats.Total)
	assert.Equal(t, int64(3), stats.Premium)
	assert.Equal(t, int64(7), stats.Free)
}

func TestAdminAnalyticsService_GetSubscriptionStats_DBError(t *testing.T) {
	db, mock := setupAnalyticsDB(t)
	rdb, rMock := redismock.NewClientMock()

	rMock.ExpectGet("analytics:subscription-stats").RedisNil()

	mock.ExpectQuery(`SELECT`).
		WillReturnError(gorm.ErrInvalidDB)

	svc := NewAdminAnalyticsService(db, rdb)
	_, err := svc.GetSubscriptionStats()
	assert.Error(t, err)
}

func TestAdminAnalyticsService_GetSubscriptionStats_CacheHit(t *testing.T) {
	db, _ := setupAnalyticsDB(t)
	rdb, rMock := redismock.NewClientMock()

	rMock.ExpectGet("analytics:subscription-stats").
		SetVal(`{"total":5,"premium":2,"free":3}`)

	svc := NewAdminAnalyticsService(db, rdb)
	stats, err := svc.GetSubscriptionStats()
	require.NoError(t, err)
	assert.Equal(t, int64(5), stats.Total)
	assert.Equal(t, int64(2), stats.Premium)
	assert.Equal(t, int64(3), stats.Free)
}
