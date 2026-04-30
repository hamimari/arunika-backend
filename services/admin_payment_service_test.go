package services

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAdminPaymentDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	dialector := postgres.New(postgres.Config{Conn: db, DriverName: "postgres"})
	gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return gormDB, mock
}

func paymentTxCols() []string {
	return []string{
		"id", "order_id", "user_id", "transaction_id",
		"transaction_status", "payment_type", "gross_amount",
		"status_code", "fraud_status", "raw_payload",
		"created_at", "updated_at",
	}
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestAdminPaymentService_List_Success(t *testing.T) {
	db, mock := setupAdminPaymentDB(t)
	svc := NewAdminPaymentService(db)

	id1 := uuid.New()
	uid := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "payment_transactions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" ORDER BY created_at DESC LIMIT $1`)).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows(paymentTxCols()).
			AddRow(id1, "sub-001", uid, "txn-001", "settlement", "gopay", "50000", "200", "accept", "{}", now, now))

	items, total, err := svc.List("", "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.Equal(t, "sub-001", items[0].OrderID)
}

func TestAdminPaymentService_List_WithStatusFilter(t *testing.T) {
	db, mock := setupAdminPaymentDB(t)
	svc := NewAdminPaymentService(db)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "payment_transactions" WHERE transaction_status = $1`)).
		WithArgs("settlement").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" WHERE transaction_status = $1 ORDER BY created_at DESC LIMIT $2`)).
		WithArgs("settlement", 20).
		WillReturnRows(sqlmock.NewRows(paymentTxCols()))

	items, total, err := svc.List("settlement", "", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, items)
}

func TestAdminPaymentService_List_WithSearchFilter(t *testing.T) {
	db, mock := setupAdminPaymentDB(t)
	svc := NewAdminPaymentService(db)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "payment_transactions" WHERE order_id ILIKE $1 OR transaction_id ILIKE $2`)).
		WithArgs("%sub-001%", "%sub-001%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" WHERE order_id ILIKE $1 OR transaction_id ILIKE $2 ORDER BY created_at DESC LIMIT $3`)).
		WithArgs("%sub-001%", "%sub-001%", 20).
		WillReturnRows(sqlmock.NewRows(paymentTxCols()))

	items, total, err := svc.List("", "sub-001", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, items)
}

func TestAdminPaymentService_List_DBError(t *testing.T) {
	db, mock := setupAdminPaymentDB(t)
	svc := NewAdminPaymentService(db)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "payment_transactions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" ORDER BY created_at DESC LIMIT $1`)).
		WithArgs(20).
		WillReturnError(gorm.ErrInvalidDB)
	_, _, err := svc.List("", "", 1, 20)
	assert.Error(t, err)
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestAdminPaymentService_Get_Success(t *testing.T) {
	db, mock := setupAdminPaymentDB(t)
	svc := NewAdminPaymentService(db)

	id1 := uuid.New()
	uid := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" WHERE id = $1 ORDER BY "payment_transactions"."id" LIMIT $2`)).
		WithArgs(id1.String(), 1).
		WillReturnRows(sqlmock.NewRows(paymentTxCols()).
			AddRow(id1, "sub-001", uid, "txn-001", "settlement", "gopay", "50000", "200", "accept", "{}", now, now))

	item, err := svc.Get(id1.String())
	require.NoError(t, err)
	assert.Equal(t, "sub-001", item.OrderID)
}

func TestAdminPaymentService_Get_NotFound(t *testing.T) {
	db, mock := setupAdminPaymentDB(t)
	svc := NewAdminPaymentService(db)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" WHERE id = $1 ORDER BY "payment_transactions"."id" LIMIT $2`)).
		WithArgs("non-existent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.Get("non-existent")
	assert.Error(t, err)
}
