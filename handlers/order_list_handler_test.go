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
)

func TestOrderHandler_List_ReturnsEnrichedHistory(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewOrderService(gormDB, services.NewProductService(gormDB))
	paymentSvc := services.NewPaymentService(gormDB, services.NewEntitlementService(gormDB))
	h := NewOrderHandler(svc, paymentSvc)

	userID := uuid.New()
	arOrder, dongengOrder, pkgOrder := uuid.New(), uuid.New(), uuid.New()
	arProduct, dongengProduct, pkgID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now()
	old := now.Add(-72 * time.Hour) // too old to trigger a Midtrans sync

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "orders" WHERE user_id = $1`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`)).
		WithArgs(userID, 20).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(arOrder, userID, arProduct, nil, 15000, "PAID", now, now).
			AddRow(dongengOrder, userID, dongengProduct, nil, 10000, "FAILED", now, now).
			AddRow(pkgOrder, userID, nil, pkgID, 49000, "PENDING", old, old))

	mock.ExpectQuery(`FROM product_ar_cards pac JOIN ar_cards a`).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "title"}).AddRow(arProduct.String(), "Kartu Harimau"))
	mock.ExpectQuery(`FROM product_dongengs pd JOIN dongengs d`).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "title"}).AddRow(dongengProduct.String(), "Kancil"))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id IN ($1)`)).
		WithArgs(pkgID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type"}).AddRow(pkgID.String(), "Paket Hewan", "content"))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "order_id","payment_type","raw_payload","created_at" FROM "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"order_id", "payment_type", "raw_payload", "created_at"}).
			// newest row is a status sync without bank detail; older webhook row names the bank
			AddRow(arOrder, "bank_transfer", `{"transaction_status":"settlement"}`, now).
			AddRow(arOrder, "bank_transfer", `{"va_numbers":[{"bank":"bni"}]}`, now.Add(-time.Minute)).
			AddRow(dongengOrder, "gopay", `{}`, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orders", nil)
	c.Set("userID", userID.String())

	h.List(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Data  []services.UserOrderView `json:"data"`
		Total int64                    `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.EqualValues(t, 3, resp.Total)
	require.Len(t, resp.Data, 3)

	assert.Equal(t, "AR_CARD", resp.Data[0].ItemType)
	assert.Equal(t, "Kartu Harimau", resp.Data[0].ItemName)
	assert.Equal(t, "BNI Virtual Account", resp.Data[0].PaymentMethod)

	assert.Equal(t, "DONGENG", resp.Data[1].ItemType)
	assert.Equal(t, "GoPay", resp.Data[1].PaymentMethod)
	assert.Equal(t, "FAILED", resp.Data[1].Status)

	assert.Equal(t, "PACKAGE", resp.Data[2].ItemType)
	assert.Equal(t, "Paket Hewan", resp.Data[2].ItemName)
	assert.Equal(t, "", resp.Data[2].PaymentMethod)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderHandler_List_Unauthorized(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	h := NewOrderHandler(services.NewOrderService(gormDB, services.NewProductService(gormDB)),
		services.NewPaymentService(gormDB, services.NewEntitlementService(gormDB)))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/orders", nil)
	h.List(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
