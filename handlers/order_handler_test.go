package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func orderCols() []string {
	return []string{"id", "user_id", "product_id", "package_id", "amount_idr", "status", "created_at", "updated_at"}
}

func TestOrderHandler_GetByID_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewOrderService(gormDB, services.NewProductService(gormDB))
	paymentSvc := services.NewPaymentService(gormDB, services.NewEntitlementService(gormDB))
	h := NewOrderHandler(svc, paymentSvc)

	orderID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, uuid.New(), 29000, "PAID", now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: orderID.String()}}
	c.Set("userID", userID.String())

	h.GetByID(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
}

func TestOrderHandler_GetByID_PendingOrder_SyncFailsOpen(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewOrderService(gormDB, services.NewProductService(gormDB))
	paymentSvc := services.NewPaymentService(gormDB, services.NewEntitlementService(gormDB))
	h := NewOrderHandler(svc, paymentSvc)

	os.Unsetenv("MIDTRANS_SERVER_KEY")

	orderID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, uuid.New(), 29000, "PENDING", now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: orderID.String()}}
	c.Set("userID", userID.String())

	// No MIDTRANS_SERVER_KEY configured — SyncOrderStatus must fail open and
	// still return the last known (PENDING) DB state instead of erroring.
	h.GetByID(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "PENDING", data["status"])
}

func TestOrderHandler_GetByID_NotOwner_404(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewOrderService(gormDB, services.NewProductService(gormDB))
	paymentSvc := services.NewPaymentService(gormDB, services.NewEntitlementService(gormDB))
	h := NewOrderHandler(svc, paymentSvc)

	orderID := uuid.New()
	ownerID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, ownerID, nil, uuid.New(), 29000, "PAID", now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: orderID.String()}}
	c.Set("userID", uuid.New().String()) // different user

	h.GetByID(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOrderHandler_GetByID_UnknownOrder_404(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewOrderService(gormDB, services.NewProductService(gormDB))
	paymentSvc := services.NewPaymentService(gormDB, services.NewEntitlementService(gormDB))
	h := NewOrderHandler(svc, paymentSvc)

	orderID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2`)).
		WithArgs(orderID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: orderID.String()}}
	c.Set("userID", uuid.New().String())

	h.GetByID(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOrderHandler_GetByID_Unauthorized(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewOrderService(gormDB, services.NewProductService(gormDB))
	paymentSvc := services.NewPaymentService(gormDB, services.NewEntitlementService(gormDB))
	h := NewOrderHandler(svc, paymentSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orders/"+uuid.New().String(), nil)
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
	// no userID set

	h.GetByID(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOrderHandler_GetByID_InvalidOrderID(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewOrderService(gormDB, services.NewProductService(gormDB))
	paymentSvc := services.NewPaymentService(gormDB, services.NewEntitlementService(gormDB))
	h := NewOrderHandler(svc, paymentSvc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orders/not-a-uuid", nil)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}
	c.Set("userID", uuid.New().String())

	h.GetByID(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
