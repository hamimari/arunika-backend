package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"arunika_backend/services"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupHandlerDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	dialector := postgres.New(postgres.Config{Conn: db, DriverName: "postgres"})
	gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return gormDB, mock
}

// ─── DongengHandler ───────────────────────────────────────────────────────────

func TestDongengHandler_GetFairyTales_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewDongengService(gormDB)
	h := NewDongengHandler(svc)

	id1 := uuid.New()
	now := time.Now()

	countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnRows(countRows)

	rows := sqlmock.NewRows([]string{
		"id", "title", "age_start", "age_end", "image_url", "audio_url",
		"is_free", "category_id", "duration", "created_at", "updated_at", "is_deleted",
	}).AddRow(id1, "Kancil", 3, 6, "https://img/k.png", "", true, nil, int64(300), now, now, false)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE is_deleted = $1 LIMIT $2`)).
		WithArgs(false, 10).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/fairy-tales", nil)

	h.GetFairyTales(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
}

func TestDongengHandler_GetFairyTales_DBError(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewDongengService(gormDB)
	h := NewDongengHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/fairy-tales", nil)

	h.GetFairyTales(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDongengHandler_GetFairyTaleByID_NotFound(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewDongengService(gormDB)
	h := NewDongengHandler(svc)

	mock.ExpectQuery(`SELECT \* FROM "dongengs" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/fairy-tales/missing-id", nil)
	c.Params = gin.Params{{Key: "id", Value: "missing-id"}}

	h.GetFairyTaleByID(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── ArHandler ────────────────────────────────────────────────────────────────

func TestArHandler_FindById_NotFound(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewArService(gormDB)
	h := NewArHandler(svc)

	mock.ExpectQuery(`SELECT \* FROM "ar_cards" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ar/cards/bad-id", nil)
	c.Params = gin.Params{{Key: "id", Value: "bad-id"}}

	h.FindById(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestArHandler_FindById_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewArService(gormDB)
	h := NewArHandler(svc)

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code", "created_at", "expires_at",
	}).AddRow("card-1", "model", "Dragon", "https://cdn/dragon.glb", "", "DRG", now, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 ORDER BY "ar_cards"."id" LIMIT $2`)).
		WithArgs("card-1", 1).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ar/cards/card-1", nil)
	c.Params = gin.Params{{Key: "id", Value: "card-1"}}

	h.FindById(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── CategoryHandler ──────────────────────────────────────────────────────────

func TestCategoryHandler_GetCategories_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewCategoryService(gormDB)
	h := NewCategoryHandler(svc)

	id := uuid.New()
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "name", "image_url", "created_at", "updated_at", "is_deleted",
	}).AddRow(id, "Fabel", "https://img/f.png", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "categories" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/categories", nil)

	h.GetCategories(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCategoryHandler_GetCategories_DBError(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewCategoryService(gormDB)
	h := NewCategoryHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "categories" WHERE is_deleted = $1`)).
		WithArgs(false).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/categories", nil)

	h.GetCategories(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─── UserHandler ──────────────────────────────────────────────────────────────

func TestUserHandler_GetUserByID_Forbidden(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewUserService(gormDB)
	h := NewUserHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/user/other-user-id", nil)
	c.Params = gin.Params{{Key: "id", Value: "other-user-id"}}
	c.Set("userID", "current-user-id") // different from param

	h.GetUserByID(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserHandler_UpdateUser_InvalidInput(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewUserService(gormDB)
	h := NewUserHandler(svc)

	body := `{"name": ""}` // missing required fields
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/user", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateUser(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── AuthHandler ──────────────────────────────────────────────────────────────

func TestAuthHandler_Login_InvalidInput(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	body := `{}` // missing email and password
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_SignUp_InvalidInput(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	body := `{"name": ""}` // missing required fields
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/signup", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SignUp(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_ForgotPassword_InvalidEmail(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	body := `{}` // no email
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ForgotPassword(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_ResetPassword_MissingToken(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	body := `{"new_password": "newpass"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	// no token query param

	h.ResetPassword(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── AnimalHandler ────────────────────────────────────────────────────────────

func TestAnimalHandler_GetAnimals_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAnimalService(gormDB)
	h := NewAnimalHandler(svc)

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "name", "emoji", "category", "image_url", "bg_color", "fact",
		"is_unlocked", "created_at", "updated_at", "is_deleted",
	}).
		AddRow("1", "Sapi", "🐄", "ternak", "", "#FFF8E1", "Sapi fun fact", true, now, now, false).
		AddRow("2", "Rusa", "🦌", "hutan", "", "#E8F5E9", "Rusa fun fact", true, now, now, false)

	mock.ExpectQuery(`SELECT \* FROM "animals" WHERE is_deleted`).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/animals", nil)

	h.GetAnimals(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].([]interface{})
	require.True(t, ok)
	assert.Len(t, data, 2)
}

func TestAnimalHandler_GetAnimals_DBError(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAnimalService(gormDB)
	h := NewAnimalHandler(svc)

	mock.ExpectQuery(`SELECT \* FROM "animals" WHERE is_deleted`).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/animals", nil)

	h.GetAnimals(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─── BannerHandler ────────────────────────────────────────────────────────────

func TestBannerHandler_GetActiveBanners_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewBannerService(gormDB)
	h := NewBannerHandler(svc)

	id := uuid.New()
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "title", "image_url", "type", "is_active", "sort_order",
		"cta_url", "emoji", "fact", "created_at", "updated_at", "is_deleted",
	}).AddRow(id, "Promo", "https://cdn/p.png", "promo", true, 0, nil, nil, nil, now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "banners" WHERE is_active = $1 AND is_deleted = $2 ORDER BY sort_order ASC`,
	)).WithArgs(true, false).WillReturnRows(rows)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/banners", nil)

	h.GetActiveBanners(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].([]interface{})
	require.True(t, ok)
	assert.Len(t, data, 1)
}

func TestBannerHandler_GetActiveBanners_DBError(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewBannerService(gormDB)
	h := NewBannerHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "banners" WHERE is_active = $1 AND is_deleted = $2 ORDER BY sort_order ASC`,
	)).WithArgs(true, false).WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/banners", nil)

	h.GetActiveBanners(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─── PaymentHandler ───────────────────────────────────────────────────────────

func TestPaymentHandler_CreatePayment_MissingUserID(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewPaymentService(gormDB)
	h := NewPaymentHandler(svc)

	body := `{"plan_name":"Paket Hutan","amount":29000}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/payment/create", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	// No "userID" in context — should return 401

	h.CreatePayment(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestPaymentHandler_CreatePayment_InvalidBody(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewPaymentService(gormDB)
	h := NewPaymentHandler(svc)

	body := `{}` // missing required fields
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/payment/create", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", "user-123")

	h.CreatePayment(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPaymentHandler_PaymentCallback_InvalidBody(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewPaymentService(gormDB)
	h := NewPaymentHandler(svc)

	body := `not-json`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/payment/callback", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PaymentCallback(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
