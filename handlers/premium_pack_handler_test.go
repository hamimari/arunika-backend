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
	svc := services.NewPremiumPackService(gormDB)
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

func TestGetActivePacks_FilteredByType(t *testing.T) {
	gormDB, mock := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB)
	h := NewPremiumPackHandler(svc)

	now := time.Now()
	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow("id-5", "Bulanan", "Akses 1 bulan", 39000, "subscription", nil, false, true, 1, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE is_active = true AND type = $1 ORDER BY sort_order asc`)).
		WithArgs("subscription").
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/premium/packs?type=subscription", nil)
	c.Request.URL.RawQuery = "type=subscription"
	// simulate gin query param
	c.Params = gin.Params{}
	h.GetActivePacks(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── Admin: GET /admin/premium/packs ─────────────────────────────────────────

func TestAdminListPacks_ReturnsAll(t *testing.T) {
	gormDB, mock := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB)
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
	svc := services.NewPremiumPackService(gormDB)
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

func TestAdminCreatePack_InvalidBody(t *testing.T) {
	gormDB, _ := setupPremiumPackDB(t)
	svc := services.NewPremiumPackService(gormDB)
	h := NewPremiumPackHandler(svc)

	body := `{"name":""}` // missing required fields
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/admin/premium/packs", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.AdminCreatePack(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
