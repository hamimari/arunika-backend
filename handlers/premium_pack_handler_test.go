package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"arunika_backend/services"
)

func setupPremiumPackDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	dialector := postgres.New(postgres.Config{Conn: db, DriverName: "postgres"})
	gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return gormDB, mock
}

func premiumPackColumns() []string {
	return []string{"id", "name", "subtitle", "price_idr", "type", "badge_label", "is_best_value", "is_active", "sort_order", "created_at", "updated_at"}
}

// ─── Public: GET /premium/packs ───────────────────────────────────────────────

func TestGetActivePacks_ReturnsOnlyActive(t *testing.T) {
	gormDB, mock := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB)))
	h := NewPremiumPackHandler(svc)

	now := time.Now()
	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow("id-1", "Paket Hutan", "8 Hewan Hutan", 29000, "content", nil, false, true, 1, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE is_active = true ORDER BY sort_order asc`)).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/premium/packs", nil)
	h.GetActivePacks(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestGetActivePacks_TypeParamFilters(t *testing.T) {
	gormDB, mock := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB)))
	h := NewPremiumPackHandler(svc)

	now := time.Now()
	// Only the subscription row is returned — type query param now filters.
	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow("id-5", "Bulanan", "Akses 1 bulan", 39000, "subscription", nil, false, true, 1, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE is_active = true AND type = $1 ORDER BY sort_order asc`)).
		WithArgs("subscription").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/premium/packs?type=subscription", nil)
	c.Request.URL.RawQuery = "type=subscription"
	c.Params = gin.Params{}
	h.GetActivePacks(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 1)
}

// ─── Admin: GET /admin/premium/packs ─────────────────────────────────────────

func TestAdminListPacks_ReturnsAll(t *testing.T) {
	gormDB, mock := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB)))
	h := NewPremiumPackHandler(svc)

	now := time.Now()
	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow("id-1", "Paket Hutan", "8 Hewan Hutan", 29000, "content", nil, false, true, 1, now, now).
		AddRow("id-2", "Paket Tersembunyi", "Hidden", 0, "content", nil, false, false, 9, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" ORDER BY sort_order asc`)).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/admin/premium/packs", nil)
	h.AdminListPacks(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

// ─── Admin: POST /admin/premium/packs ────────────────────────────────────────

func TestAdminCreatePack_Success(t *testing.T) {
	gormDB, mock := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB)))
	h := NewPremiumPackHandler(svc)

	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "premium_packages"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow("new-id", now, now))
	mock.ExpectCommit()

	body := `{"name":"Test Pack","subtitle":"Test Sub","price_idr":10000,"type":"content"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/admin/premium/packs", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.AdminCreatePack(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// ─── Admin: Package Items ─────────────────────────────────────────────────────

func TestAdminListPackItems_Success(t *testing.T) {
	gormDB, mock := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB)))
	h := NewPremiumPackHandler(svc)

	packageID := "11111111-1111-1111-1111-111111111111"
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_package_items" WHERE package_id = $1`)).
		WillReturnRows(sqlmock.NewRows([]string{"package_id", "product_id", "created_at"}).
			AddRow(packageID, "22222222-2222-2222-2222-222222222222", now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/admin/premium/packs/"+packageID+"/items", nil)
	c.Params = gin.Params{{Key: "id", Value: packageID}}
	h.AdminListPackItems(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminAddPackItem_Success(t *testing.T) {
	gormDB, mock := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB)))
	h := NewPremiumPackHandler(svc)

	packageID := "11111111-1111-1111-1111-111111111111"
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "premium_package_items"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"product_id":"22222222-2222-2222-2222-222222222222"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/admin/premium/packs/"+packageID+"/items", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: packageID}}
	h.AdminAddPackItem(c)

	assert.Equal(t, http.StatusCreated, c.Writer.Status())
}

func TestAdminAddPackItem_InvalidBody(t *testing.T) {
	gormDB, _ := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB)))
	h := NewPremiumPackHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/admin/premium/packs/id-1/items", bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "id-1"}}
	h.AdminAddPackItem(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminRemovePackItem_Success(t *testing.T) {
	gormDB, mock := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB)))
	h := NewPremiumPackHandler(svc)

	packageID := "11111111-1111-1111-1111-111111111111"
	productID := "22222222-2222-2222-2222-222222222222"
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "premium_package_items" WHERE package_id = $1 AND product_id = $2`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/admin/premium/packs/"+packageID+"/items/"+productID, nil)
	c.Params = gin.Params{{Key: "id", Value: packageID}, {Key: "product_id", Value: productID}}
	h.AdminRemovePackItem(c)

	assert.Equal(t, http.StatusNoContent, c.Writer.Status())
}

func TestAdminCreatePack_InvalidBody(t *testing.T) {
	gormDB, _ := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB)))
	h := NewPremiumPackHandler(svc)

	body := `{"name":""}` // missing required fields
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/admin/premium/packs", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.AdminCreatePack(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
