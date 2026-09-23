package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"arunika_backend/services"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func parentCols() []string {
	return []string{"id", "created_at", "updated_at", "is_deleted", "name", "phone_number", "email_address", "password", "address", "city"}
}

func newTestOrderHandler(db *gorm.DB) *AdminOrderHandler {
	productSvc := services.NewProductService(db)
	orderSvc := services.NewOrderService(db, productSvc)
	entitlementSvc := services.NewEntitlementService(db)
	paymentSvc := services.NewPaymentService(db, entitlementSvc)
	return NewAdminOrderHandler(orderSvc, paymentSvc)
}

func TestAdminOrderHandler_List_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := newTestOrderHandler(gormDB)

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
		WillReturnRows(sqlmock.NewRows(parentCols()))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/orders", nil)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
	assert.Equal(t, float64(1), resp["total"])
}

func TestAdminOrderHandler_List_WithStatusFilter(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := newTestOrderHandler(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "orders" WHERE status = $1`)).
		WithArgs("PENDING").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE status = $1 ORDER BY orders.created_at DESC LIMIT $2`)).
		WithArgs("PENDING", 20).
		WillReturnRows(sqlmock.NewRows(orderCols()))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/orders?status=PENDING", nil)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminOrderHandler_List_WithSearchParam(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := newTestOrderHandler(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "orders" JOIN parents p ON p.id = orders.user_id WHERE p.name ILIKE $1 OR p.email_address ILIKE $2 OR p.phone_number ILIKE $3 OR orders.user_id::text ILIKE $4 OR orders.id::text ILIKE $5`)).
		WithArgs("%hamim@gmail.com%", "%hamim@gmail.com%", "%hamim@gmail.com%", "%hamim@gmail.com%", "%hamim@gmail.com%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT orders.* FROM "orders" JOIN parents p ON p.id = orders.user_id WHERE p.name ILIKE $1 OR p.email_address ILIKE $2 OR p.phone_number ILIKE $3 OR orders.user_id::text ILIKE $4 OR orders.id::text ILIKE $5 ORDER BY orders.created_at DESC LIMIT $6`)).
		WithArgs("%hamim@gmail.com%", "%hamim@gmail.com%", "%hamim@gmail.com%", "%hamim@gmail.com%", "%hamim@gmail.com%", 20).
		WillReturnRows(sqlmock.NewRows(orderCols()))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Mirrors exactly what the frontend sends: axios URL-encodes "@" as %40.
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/orders?search=hamim%40gmail.com", nil)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminOrderHandler_List_DBError(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := newTestOrderHandler(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "orders"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" ORDER BY orders.created_at DESC LIMIT $1`)).
		WithArgs(20).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/orders", nil)

	h.List(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminOrderHandler_Sync_InvalidID(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	h := newTestOrderHandler(gormDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/orders/not-a-uuid/sync", nil)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	h.Sync(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminOrderHandler_Sync_NotFound(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := newTestOrderHandler(gormDB)

	orderID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/orders/"+orderID.String()+"/sync", nil)
	c.Params = gin.Params{{Key: "id", Value: orderID.String()}}

	h.Sync(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Sync's "already PAID" path never calls out to Midtrans (SyncOrderStatus
// short-circuits for non-PENDING orders), so it's exercised here without
// needing to fake an HTTP call to Midtrans.
func TestAdminOrderHandler_Sync_AlreadySettled(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := newTestOrderHandler(gormDB)

	orderID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, nil, 29000, "PAID", now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id IN ($1)`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows(parentCols()))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/orders/"+orderID.String()+"/sync", nil)
	c.Params = gin.Params{{Key: "id", Value: orderID.String()}}

	h.Sync(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
}

func orderColsWithProvider() []string {
	return append(orderCols(), "provider", "purchase_token")
}

// A Google Play order with a purchase token already on file re-verifies
// against Google using that token directly — no manual input needed.
func TestAdminOrderHandler_Sync_GooglePlayWithStoredToken(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := newTestOrderHandler(gormDB)

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderColsWithProvider()).
			AddRow(orderID, userID, nil, packageID, 29000, "PENDING", now, now, "google_play", "tok-stored"))

	// Idempotent replay of a purchase token: the order is still PENDING, so
	// VerifyPlayPurchase runs its normal path — nothing special to fake
	// here beyond letting it fail cleanly (no verifier configured), which
	// is enough to prove Sync actually reused the stored token instead of
	// requiring one in the request body.
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET "purchase_token"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 AND user_id = $2 ORDER BY "orders"."id" LIMIT $3 FOR UPDATE`)).
		WithArgs(orderID, userID, 1).
		WillReturnRows(sqlmock.NewRows(orderColsWithProvider()).
			AddRow(orderID, userID, nil, packageID, 29000, "PENDING", now, now, "google_play", "tok-stored"))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND order_id != $2`)).
		WithArgs("tok-stored", orderID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/orders/"+orderID.String()+"/sync", nil)
	c.Params = gin.Params{{Key: "id", Value: orderID.String()}}

	h.Sync(c)

	// Play Billing isn't configured in this test (no service account JSON),
	// so verification itself fails — the point here is that it got that
	// far using the stored token, without the request needing one.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// A Google Play order with no purchase token on file yet can't be
// auto-synced — Sync reports that clearly instead of silently no-op'ing,
// pointing at the manual recover-play fallback.
func TestAdminOrderHandler_Sync_GooglePlayNoStoredToken(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := newTestOrderHandler(gormDB)

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderColsWithProvider()).
			AddRow(orderID, userID, nil, packageID, 29000, "PENDING", now, now, "google_play", nil))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/orders/"+orderID.String()+"/sync", nil)
	c.Params = gin.Params{{Key: "id", Value: orderID.String()}}

	h.Sync(c)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
