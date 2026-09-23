package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

	"arunika_backend/models"
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

func newTestDongengService(db *gorm.DB) *services.DongengService {
	return services.NewDongengService(db, services.NewProductService(db), services.NewEntitlementService(db))
}

func newTestArService(db *gorm.DB) *services.ArService {
	return services.NewArService(db, services.NewProductService(db), services.NewEntitlementService(db))
}

func TestDongengHandler_GetFairyTales_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := newTestDongengService(gormDB)
	h := NewDongengHandler(svc)

	id1 := uuid.New()
	now := time.Now()

	countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2`)).
		WithArgs(false, false).
		WillReturnRows(countRows)

	rows := sqlmock.NewRows([]string{
		"id", "title", "age_start", "age_end", "image_url", "audio_url",
		"is_free", "category_id", "duration", "created_at", "updated_at", "is_deleted",
	}).AddRow(id1, "Kancil", 3, 6, "https://img/k.png", "", true, nil, int64(300), now, now, false)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2 LIMIT $3`)).
		WithArgs(false, false, 10).
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
	svc := newTestDongengService(gormDB)
	h := NewDongengHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2`)).
		WithArgs(false, false).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/fairy-tales", nil)

	h.GetFairyTales(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDongengHandler_GetFairyTaleByID_NotFound(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := newTestDongengService(gormDB)
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

func TestDongengHandler_RecordPlay_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := newTestDongengService(gormDB)
	h := NewDongengHandler(svc)

	dongengID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "dongeng_play_history"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/fairy-tales/"+dongengID.String()+"/play", nil)
	c.Params = gin.Params{{Key: "id", Value: dongengID.String()}}
	c.Set("userID", uuid.New().String())

	h.RecordPlay(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// A guest (no authenticated user) has nothing to attribute a play to — this
// must be a no-op success, not a 401, so it never blocks a guest watching
// free content (see the fix for the "guest gets kicked to login" bug).
func TestDongengHandler_RecordPlay_Anonymous_NoOpSuccess(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := newTestDongengService(gormDB)
	h := NewDongengHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/fairy-tales/"+uuid.New().String()+"/play", nil)
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}

	h.RecordPlay(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDongengHandler_UpdateProgressHandler_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := newTestDongengService(gormDB)
	h := NewDongengHandler(svc)

	dongengID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "dongeng_play_history"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"progress_seconds": 55}`)
	c.Request = httptest.NewRequest(http.MethodPut, "/fairy-tales/"+dongengID.String()+"/play", body)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: dongengID.String()}}
	c.Set("userID", uuid.New().String())

	h.UpdateProgressHandler(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDongengHandler_GetHistory_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := newTestDongengService(gormDB)
	h := NewDongengHandler(svc)

	userID := uuid.New()
	dongengID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT h.dongeng_id, h.progress_seconds, d.duration AS total_seconds, h.started_at FROM dongeng_play_history h JOIN dongengs d ON d.id = h.dongeng_id AND d.is_deleted = false AND d.hidden = false WHERE h.user_id = $1 ORDER BY h.updated_at DESC LIMIT $2`)).
		WithArgs(userID, 20).
		WillReturnRows(sqlmock.NewRows([]string{"dongeng_id", "progress_seconds", "total_seconds", "started_at"}).
			AddRow(dongengID, 10, int64(120), now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/fairy-tales/history", nil)
	c.Set("userID", userID.String())

	h.GetHistory(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
}

func TestDongengHandler_GetHistory_Unauthorized(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := newTestDongengService(gormDB)
	h := NewDongengHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/fairy-tales/history", nil)

	h.GetHistory(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─── ArHandler ────────────────────────────────────────────────────────────────

func TestArHandler_FindById_NotFound(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := newTestArService(gormDB)
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
	svc := newTestArService(gormDB)
	h := NewArHandler(svc)

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code", "created_at", "expires_at",
	}).AddRow("card-1", "model", "Dragon", "https://cdn/dragon.glb", "", "DRG", now, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 AND hidden = $2 ORDER BY "ar_cards"."id" LIMIT $3`)).
		WithArgs("card-1", false, 1).
		WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_ar_cards" WHERE ar_card_id = $1 ORDER BY "product_ar_cards"."product_id" LIMIT $2`)).
		WithArgs("card-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}))

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

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "categories" WHERE is_deleted = $1 AND hidden = $2`)).
		WithArgs(false, false).
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

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "categories" WHERE is_deleted = $1 AND hidden = $2`)).
		WithArgs(false, false).
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
	h := NewUserHandler(svc, nil, nil)

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
	h := NewUserHandler(svc, nil, nil)

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

func TestAuthHandler_ForgotPassword_UnknownEmail_StillReturns200(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("nobody@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted",
		}))

	body := `{"email": "nobody@example.com"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/forgot-password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ForgotPassword(c)

	// Must be indistinguishable from a real send — this is what actually
	// closes the account-enumeration hole, not just the service-level fix.
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["message"], "nobody@example.com")
	assert.NotContains(t, w.Body.String(), "not found")
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

func TestAuthHandler_ResetPassword_PasswordTooShort(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	body := `{"new_password": "short"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/reset-password?token=whatever", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.ResetPassword(c)

	// Rejected before ever touching the token/DB — a direct API call can't
	// bypass the same 8-character minimum the web page enforces client-side.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthHandler_ResetPasswordPage_ServesTheHTMLPage(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	require.NoError(t, err)
	prevWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoRoot))
	defer os.Chdir(prevWd)

	svc := services.NewAuthService(nil, nil)
	h := NewAuthHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/reset-password?token=abc", nil)

	h.ResetPasswordPage(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Reset Password")
}

// ─── AdminUserHandler ─────────────────────────────────────────────────────────

func TestAdminUserHandler_ListUsers_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAdminUserService(gormDB)
	h := NewAdminUserHandler(svc)

	uid := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "parents" WHERE is_deleted = false`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE is_deleted = false ORDER BY created_at DESC LIMIT $1`)).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email_address", "city", "created_at", "updated_at", "is_deleted"}).
			AddRow(uid, "Dewi", "dewi@test.com", "Surabaya", now, now, false))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/users", nil)

	h.ListUsers(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
}

func TestAdminUserHandler_GetUserDetail_NotFound(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAdminUserService(gormDB)
	h := NewAdminUserHandler(svc)

	mock.ExpectQuery(`SELECT \* FROM "parents" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/users/bad-id", nil)
	c.Params = gin.Params{{Key: "id", Value: "bad-id"}}

	h.GetUserDetail(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminUserHandler_UpdatePermission_InvalidBody(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewAdminUserService(gormDB)
	h := NewAdminUserHandler(svc)

	body := `{}` // missing required action field
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/users/some-id/permission", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "some-id"}}

	h.UpdatePermission(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserHandler_UpdatePermission_InvalidAction(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewAdminUserService(gormDB)
	h := NewAdminUserHandler(svc)

	body := `{"action": "promote"}` // not "grant" or "revoke"
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/users/some-id/permission", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "some-id"}}

	h.UpdatePermission(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUserHandler_UpdatePermission_GrantInvalidUUID(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewAdminUserService(gormDB)
	h := NewAdminUserHandler(svc)

	body := `{"action": "grant", "duration_days": 30}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/users/not-a-uuid/permission", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	h.UpdatePermission(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─── BannerHandler ────────────────────────────────────────────────────────────

func TestBannerHandler_List_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewBannerService(gormDB)
	h := NewBannerHandler(svc)

	now := time.Now()
	id := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "banners" WHERE is_deleted = false`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "banners" WHERE is_deleted = false`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "image_url", "is_active", "hidden", "sort_order", "created_at", "updated_at", "is_deleted"}).
			AddRow(id, "Promo", "https://img/promo.png", true, false, 1, now, now, false))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/content/banners", nil)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
}

func TestBannerHandler_Get_NotFound(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewBannerService(gormDB)
	h := NewBannerHandler(svc)

	mock.ExpectQuery(`SELECT \* FROM "banners" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/content/banners/missing", nil)
	c.Params = gin.Params{{Key: "id", Value: "missing"}}

	h.Get(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBannerHandler_Delete_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewBannerService(gormDB)
	h := NewBannerHandler(svc)

	bannerID := uuid.New().String()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "banners" SET "is_deleted"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), bannerID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/admin/content/banners/"+bannerID, nil)
	c.Params = gin.Params{{Key: "id", Value: bannerID}}

	h.Delete(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBannerHandler_ToggleActive_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewBannerService(gormDB)
	h := NewBannerHandler(svc)

	bannerID := uuid.New().String()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "banners" SET "is_active"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), bannerID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"is_active": true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/content/banners/"+bannerID+"/active", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: bannerID}}

	h.ToggleActive(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["is_active"])
}

// ─── AdminContentHandler ──────────────────────────────────────────────────────

// helper for content handler tests
func setupContentHandler(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *AdminContentHandler) {
	t.Helper()
	db, mock := setupHandlerDB(t)
	svc := services.NewAdminContentService(db)
	return db, mock, NewAdminContentHandler(svc)
}

// ── FairyTale CRUD ──

func TestAdminContentHandler_CreateFairyTale_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "dongengs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	body := `{"title":"Bawang Merah","image_url":"https://img/b.png","is_free":true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/content/fairy-tales", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CreateFairyTale(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminContentHandler_DeleteFairyTale_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "dongengs" SET "is_deleted"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/admin/content/fairy-tales/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.DeleteFairyTale(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminContentHandler_ToggleFairyTaleVisibility_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "dongengs" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(false, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"hidden":false}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/content/fairy-tales/"+id+"/visibility", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.ToggleFairyTaleVisibility(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── AR Card CRUD ──

func TestAdminContentHandler_CreateArCard_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "ar_cards"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"type":"alphabet","title":"A","file_url":"https://ar/a.glb"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/content/ar-cards", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CreateArCard(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminContentHandler_DeleteArCard_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := "ar-id-123"
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "ar_cards" WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/admin/content/ar-cards/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.DeleteArCard(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminContentHandler_ToggleArCardVisibility_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := "ar-id-123"
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "ar_cards" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"hidden":true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/content/ar-cards/"+id+"/visibility", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.ToggleArCardVisibility(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── Badge CRUD ──

func TestAdminContentHandler_CreateBadge_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "badges"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	body := `{"feature":"tracing","level":"beginner","threshold":5}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/content/badges", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CreateBadge(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminContentHandler_DeleteBadge_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "badges" WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/admin/content/badges/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.DeleteBadge(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminContentHandler_ToggleBadgeVisibility_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "badges" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"hidden":true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/content/badges/"+id+"/visibility", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.ToggleBadgeVisibility(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── Category CRUD ──

func TestAdminContentHandler_CreateCategory_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "categories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	body := `{"name":"Legenda","image_url":"https://img/legenda.png"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/content/categories", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CreateCategory(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminContentHandler_DeleteCategory_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "categories" SET "is_deleted"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/admin/content/categories/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.DeleteCategory(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminContentHandler_ToggleCategoryVisibility_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "categories" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(false, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"hidden":false}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/content/categories/"+id+"/visibility", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.ToggleCategoryVisibility(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── TracingItem CRUD ──

func TestAdminContentHandler_CreateTracingItem_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "tracing_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	body := `{"label":"A","type":"alphabet","guide_path_json":"[]","difficulty":1}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/content/tracing-items", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CreateTracingItem(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminContentHandler_DeleteTracingItem_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "tracing_items" WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/admin/content/tracing-items/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.DeleteTracingItem(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminContentHandler_ToggleTracingItemVisibility_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "tracing_items" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"hidden":true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/content/tracing-items/"+id+"/visibility", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.ToggleTracingItemVisibility(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── CountingQuestion CRUD ──

func TestAdminContentHandler_CreateCountingQuestion_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "counting_questions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	body := `{"level":"easy","question_json":"{}","answer":3}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/content/counting-questions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CreateCountingQuestion(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminContentHandler_DeleteCountingQuestion_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "counting_questions" WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/admin/content/counting-questions/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.DeleteCountingQuestion(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminContentHandler_ToggleCountingQuestionVisibility_Success(t *testing.T) {
	_, mock, h := setupContentHandler(t)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "counting_questions" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(false, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"hidden":false}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/content/counting-questions/"+id+"/visibility", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.ToggleCountingQuestionVisibility(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── Banner Create/Update handler tests ──

func TestBannerHandler_Create_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewBannerService(gormDB)
	h := NewBannerHandler(svc)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "banners"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	body := `{"title":"Promo","image_url":"https://img/promo.png","is_active":true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/content/banners", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// ─── AdminPaymentHandler ──────────────────────────────────────────────────────

func TestAdminPaymentHandler_List_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAdminPaymentService(gormDB)
	h := NewAdminPaymentHandler(svc)

	id1 := uuid.New()
	orderID := uuid.New()
	uid := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	cols := []string{
		"id", "order_id", "provider_order_id", "user_id", "transaction_id",
		"transaction_status", "payment_type", "gross_amount",
		"status_code", "fraud_status", "raw_payload",
		"created_at", "updated_at",
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" ORDER BY created_at DESC LIMIT $1`)).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(id1, orderID, "order-001", uid, "txn-001", "settlement", "gopay", "50000", "200", "accept", "{}", now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/payments", nil)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
	assert.Equal(t, float64(1), resp["total"])
}

func TestAdminPaymentHandler_List_DBError(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAdminPaymentService(gormDB)
	h := NewAdminPaymentHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" ORDER BY created_at DESC LIMIT $1`)).
		WithArgs(20).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/payments", nil)

	h.List(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminPaymentHandler_Get_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAdminPaymentService(gormDB)
	h := NewAdminPaymentHandler(svc)

	id1 := uuid.New()
	orderID := uuid.New()
	uid := uuid.New()
	now := time.Now()

	cols := []string{
		"id", "order_id", "provider_order_id", "user_id", "transaction_id",
		"transaction_status", "payment_type", "gross_amount",
		"status_code", "fraud_status", "raw_payload",
		"created_at", "updated_at",
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE id = $1 ORDER BY "payments"."id" LIMIT $2`)).
		WithArgs(id1.String(), 1).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(id1, orderID, "order-001", uid, "txn-001", "settlement", "gopay", "50000", "200", "accept", "{}", now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/payments/"+id1.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: id1.String()}}

	h.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
}

func TestAdminPaymentHandler_Get_NotFound(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAdminPaymentService(gormDB)
	h := NewAdminPaymentHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE id = $1 ORDER BY "payments"."id" LIMIT $2`)).
		WithArgs("non-existent", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/payments/non-existent", nil)
	c.Params = gin.Params{{Key: "id", Value: "non-existent"}}

	h.Get(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBannerHandler_ToggleVisibility_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewBannerService(gormDB)
	h := NewBannerHandler(svc)

	bannerID := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "banners" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), bannerID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body := `{"hidden":true}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/content/banners/"+bannerID+"/visibility", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: bannerID}}

	h.ToggleVisibility(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Regression: signup used to discard the error from GenerateJwtToken and
// return 201 with empty token/refresh_token. The app refuses to proceed on an
// empty token, so the user saw a generic failure for an account that had in
// fact been created, and retrying hit "email address already taken" — with
// nothing in the logs to explain it.
func TestAuthHandler_SignUp_TokenGenerationFails_Returns500(t *testing.T) {
	// GenerateJWT reads JWT_SECRET at call time and errors when it is empty,
	// which fails token issuance without needing the account insert to fail.
	t.Setenv("JWT_SECRET", "")

	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	parentCols := []string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}
	// Email not taken.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1`)).
		WillReturnRows(sqlmock.NewRows(parentCols))
	// Phone not taken.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE phone_number = $1`)).
		WillReturnRows(sqlmock.NewRows(parentCols))
	// The account itself is created successfully — that is the whole point.
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "parents"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "children"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	body := `{
		"name": "New User",
		"phone_number": "081",
		"email_address": "new@example.com",
		"address": "Jl.",
		"city": "Jakarta",
		"password": "secret123",
		"child": [{"name": "Budi", "gender": "M", "date_of_birth": "2020-01-02T00:00:00.000"}]
	}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/signup", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SignUp(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code,
		"a session that could not be issued must not be reported as 201 Created")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// The caller is told what actually happened and what to do about it.
	errMsg, _ := resp["error"].(string)
	assert.Contains(t, errMsg, "sign in")
	// And is never handed a payload carrying empty credentials.
	assert.NotContains(t, resp, "data")
	assert.NotContains(t, w.Body.String(), `"token":""`)
}

// ─── Email verification ───────────────────────────────────────────────────────

// chdirRepoRoot points the process at the project root so handlers that parse
// templates by relative path resolve them, matching how the binary runs
// (Dockerfile sets WORKDIR /app and copies templates alongside).
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	repoRoot, err := filepath.Abs("..")
	require.NoError(t, err)
	prevWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoRoot))
	t.Cleanup(func() { _ = os.Chdir(prevWd) })
}

func TestAuthHandler_VerifyEmail_InvalidToken_RendersInvalidPageNotAnError(t *testing.T) {
	chdirRepoRoot(t)
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "email_verification_tokens" WHERE token = $1`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/verify-email?token=bogus", nil)

	h.VerifyEmail(c)

	// A dead link is a page with a way forward, not an HTTP error — the user
	// opened it from their mail client and needs to be told what to do.
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Tautan tidak berlaku")
	assert.Contains(t, w.Body.String(), "Kirim ulang")
	// The token must never be echoed into the page.
	assert.NotContains(t, w.Body.String(), "bogus")
}

func TestAuthHandler_VerifyEmail_SetsNoReferrerHeader(t *testing.T) {
	chdirRepoRoot(t)
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "email_verification_tokens" WHERE token = $1`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/verify-email?token=abc", nil)

	h.VerifyEmail(c)

	// The token rides in the query string, so the page must not leak it
	// onward via Referer.
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
}

func TestAuthHandler_ResendVerification_Unauthenticated_Returns401(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/resend-verification", nil)

	h.ResendVerification(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthHandler_ResendVerification_AlreadyVerified_Returns200(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewAuthService(gormDB, nil)
	h := NewAuthHandler(svc)

	userID := uuid.New()
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted", "email_verified",
		}).AddRow(userID, "Budi", "081", "b@example.com", "hash", "Jl.", "Jakarta", now, now, false, true))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/resend-verification", nil)
	c.Set("userID", userID.String())

	h.ResendVerification(c)

	// Nothing to do is success: the caller's goal is already met.
	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet(), "no token should have been issued")
}

// userResponse embeds *models.Parent, so its serialization is the contract
// the app sees. Two things must hold: verification state is exposed, and the
// password hash is not.
func TestUserResponse_ExposesVerificationState_AndNeverThePasswordHash(t *testing.T) {
	for _, tc := range []struct {
		name     string
		verified bool
	}{
		{"verified account", true},
		{"unverified account", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(userResponse{
				Parent: &models.Parent{
					Name:          "Budi",
					EmailAddress:  "budi@example.com",
					Password:      "$2a$10$averysecretbcrypthash",
					EmailVerified: tc.verified,
				},
			})
			require.NoError(t, err)

			var got map[string]interface{}
			require.NoError(t, json.Unmarshal(payload, &got))

			require.Contains(t, got, "email_verified")
			assert.Equal(t, tc.verified, got["email_verified"])

			// The bcrypt hash must never leave the server: GET /user/:id used
			// to return it, and the app persisted it in local storage.
			assert.NotContains(t, got, "password")
			assert.NotContains(t, string(payload), "averysecretbcrypthash")
		})
	}
}
