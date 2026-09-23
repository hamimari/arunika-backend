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

// ─── GetPaymentMetrics ────────────────────────────────────────────────────────

// These replace the characterization tests written when GetPaymentMetrics
// still read user_subscriptions. It now reports real order data, so they
// assert order statuses and summed amount_idr instead.

func TestAdminAnalyticsService_GetPaymentMetrics_NoDateRange_GroupsOrdersByStatus(t *testing.T) {
	db, mock := setupAnalyticsDB(t)
	rdb, rMock := redismock.NewClientMock()

	rMock.ExpectGet("analytics:payments::").RedisNil()
	rMock.ExpectSet("analytics:payments::", sqlmock.AnyArg(), 0).SetVal("OK")

	// Must read `orders`, not `user_subscriptions`.
	mock.ExpectQuery(`FROM orders`).
		WithArgs().
		WillReturnRows(sqlmock.NewRows([]string{"status", "count", "total"}).
			AddRow("PAID", 12, int64(1_200_000)).
			AddRow("PENDING", 3, int64(150_000)).
			AddRow("REFUNDED", 1, int64(50_000)))

	svc := NewAdminAnalyticsService(db, rdb)
	rows, err := svc.GetPaymentMetrics("", "")

	require.NoError(t, err)
	require.Len(t, rows, 3)

	// Order statuses, never subscription states like "premium".
	assert.Equal(t, "PAID", rows[0].Status)
	assert.Equal(t, int64(12), rows[0].Count)
	// Gross value is summed, no longer hardcoded to zero.
	assert.Equal(t, int64(1_200_000), rows[0].Total)

	assert.Equal(t, "PENDING", rows[1].Status)
	// REFUNDED is reported separately, so the PAID group already excludes it.
	assert.Equal(t, "REFUNDED", rows[2].Status)
	assert.Equal(t, int64(50_000), rows[2].Total)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminAnalyticsService_GetPaymentMetrics_WithDateRange_BindsBothBounds(t *testing.T) {
	db, mock := setupAnalyticsDB(t)
	rdb, rMock := redismock.NewClientMock()

	rMock.ExpectGet("analytics:payments:2026-01-01:2026-01-31").RedisNil()
	rMock.ExpectSet("analytics:payments:2026-01-01:2026-01-31", sqlmock.AnyArg(), 0).SetVal("OK")

	mock.ExpectQuery(`FROM orders\s+WHERE created_at >= \$1 AND created_at <= \$2`).
		WithArgs("2026-01-01", "2026-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"status", "count", "total"}).
			AddRow("PAID", 2, int64(80_000)))

	svc := NewAdminAnalyticsService(db, rdb)
	rows, err := svc.GetPaymentMetrics("2026-01-01", "2026-01-31")

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "PAID", rows[0].Status)
	assert.Equal(t, int64(80_000), rows[0].Total)
	require.NoError(t, mock.ExpectationsWereMet())
}

// An empty orders table must yield an empty result rather than a row of
// zeroes — COALESCE guards the SUM, but GROUP BY yields no rows at all.
func TestAdminAnalyticsService_GetPaymentMetrics_NoOrders_ReturnsEmpty(t *testing.T) {
	db, mock := setupAnalyticsDB(t)
	rdb, rMock := redismock.NewClientMock()

	rMock.ExpectGet("analytics:payments::").RedisNil()
	rMock.ExpectSet("analytics:payments::", sqlmock.AnyArg(), 0).SetVal("OK")

	mock.ExpectQuery(`FROM orders`).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count", "total"}))

	svc := NewAdminAnalyticsService(db, rdb)
	rows, err := svc.GetPaymentMetrics("", "")

	require.NoError(t, err)
	assert.Empty(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}
