package services

import (
	"arunika_backend/models"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func parentCols() []string {
	return []string{"id", "created_at", "updated_at", "is_deleted", "name", "phone_number", "email_address", "password", "address", "city"}
}

func premiumPackageCols() []string {
	return []string{"id", "name", "subtitle", "price_idr", "type", "badge_label", "is_best_value", "is_active", "sort_order", "duration_days", "created_at", "updated_at"}
}

func TestOrderService_List_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewOrderService(gormDB, NewProductService(gormDB))

	id1 := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "orders"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" ORDER BY orders.created_at DESC LIMIT $1`)).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(id1, userID, nil, nil, 29000, "PAID", now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id IN ($1)`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows(parentCols()).
			AddRow(userID, now, now, false, "Budi", "0812", "budi@mail.com", "hash", "Jl. A", "Jakarta"))

	items, total, err := svc.List("", "", 1, 20)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "Budi", items[0].UserName)
	assert.Equal(t, "budi@mail.com", items[0].UserEmail)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_List_WithStatusFilter(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewOrderService(gormDB, NewProductService(gormDB))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "orders" WHERE status = $1`)).
		WithArgs("PENDING").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE status = $1 ORDER BY orders.created_at DESC LIMIT $2`)).
		WithArgs("PENDING", 20).
		WillReturnRows(sqlmock.NewRows(orderCols()))

	items, total, err := svc.List("PENDING", "", 1, 20)

	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, items)
}

func TestOrderService_List_WithSearch(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewOrderService(gormDB, NewProductService(gormDB))

	id1 := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "orders" JOIN parents p ON p.id = orders.user_id WHERE p.name ILIKE $1 OR p.email_address ILIKE $2 OR p.phone_number ILIKE $3 OR orders.user_id::text ILIKE $4 OR orders.id::text ILIKE $5`)).
		WithArgs("%budi%", "%budi%", "%budi%", "%budi%", "%budi%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT orders.* FROM "orders" JOIN parents p ON p.id = orders.user_id WHERE p.name ILIKE $1 OR p.email_address ILIKE $2 OR p.phone_number ILIKE $3 OR orders.user_id::text ILIKE $4 OR orders.id::text ILIKE $5 ORDER BY orders.created_at DESC LIMIT $6`)).
		WithArgs("%budi%", "%budi%", "%budi%", "%budi%", "%budi%", 20).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(id1, userID, nil, packageID, 99000, "PAID", now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id IN ($1)`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows(parentCols()).
			AddRow(userID, now, now, false, "Budi", "0812", "budi@mail.com", "hash", "Jl. A", "Jakarta"))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id IN ($1)`)).
		WithArgs(packageID.String()).
		WillReturnRows(sqlmock.NewRows(premiumPackageCols()).
			AddRow(packageID.String(), "Paket Hutan", "sub", 49000, "content", nil, false, true, 0, nil, now, now))

	items, total, err := svc.List("", "budi", 1, 20)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].PackageName)
	assert.Equal(t, "Paket Hutan", *items[0].PackageName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_PurchasedPackageIDs_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewOrderService(gormDB, NewProductService(gormDB))

	userID := uuid.New()
	packageID := uuid.New().String()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "package_id" FROM "orders" WHERE user_id = $1 AND status = $2 AND package_id IS NOT NULL`)).
		WithArgs(userID, "PAID").
		WillReturnRows(sqlmock.NewRows([]string{"package_id"}).AddRow(packageID))

	set, err := svc.PurchasedPackageIDs(userID)

	require.NoError(t, err)
	assert.True(t, set[packageID])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_PurchasedPackageIDs_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewOrderService(gormDB, NewProductService(gormDB))

	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "package_id" FROM "orders" WHERE user_id = $1 AND status = $2 AND package_id IS NOT NULL`)).
		WithArgs(userID, "PAID").
		WillReturnError(gorm.ErrInvalidDB)

	set, err := svc.PurchasedPackageIDs(userID)

	assert.Error(t, err)
	assert.Nil(t, set)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_GetOwnedOrder_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewOrderService(gormDB, NewProductService(gormDB))

	orderID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, nil, 29000, "PAID", now, now))

	order, err := svc.GetOwnedOrder(orderID, userID)

	require.NoError(t, err)
	assert.Equal(t, orderID, order.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderService_GetOwnedOrder_WrongUser(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewOrderService(gormDB, NewProductService(gormDB))

	orderID := uuid.New()
	ownerID := uuid.New()
	otherUserID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, ownerID, nil, nil, 29000, "PAID", now, now))

	order, err := svc.GetOwnedOrder(orderID, otherUserID)

	assert.ErrorIs(t, err, ErrOrderForbidden)
	assert.Nil(t, order)
}

func TestOrderService_GetOwnedOrder_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewOrderService(gormDB, NewProductService(gormDB))

	orderID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	order, err := svc.GetOwnedOrder(orderID, uuid.New())

	assert.Error(t, err)
	assert.Nil(t, order)
}

func TestOrderService_EnrichOne_NoProductOrPackage(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewOrderService(gormDB, NewProductService(gormDB))

	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id IN ($1)`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows(parentCols()).
			AddRow(userID, now, now, false, "Budi", "0812", "budi@mail.com", "hash", "Jl. A", "Jakarta"))

	order := models.Order{ID: uuid.New(), UserID: userID, AmountIdr: 29000, Status: "PAID", CreatedAt: now, UpdatedAt: now}
	view, err := svc.EnrichOne(order)

	require.NoError(t, err)
	assert.Equal(t, "Budi", view.UserName)
	assert.Nil(t, view.ProductName)
	assert.Nil(t, view.PackageName)
	assert.NoError(t, mock.ExpectationsWereMet())
}
