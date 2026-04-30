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
	uid := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "payment_transactions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	cols := []string{
		"id", "order_id", "user_id", "transaction_id",
		"transaction_status", "payment_type", "gross_amount",
		"status_code", "fraud_status", "raw_payload",
		"created_at", "updated_at",
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" ORDER BY created_at DESC LIMIT $1`)).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(id1, "sub-001", uid, "txn-001", "settlement", "gopay", "50000", "200", "accept", "{}", now, now))

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

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "payment_transactions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" ORDER BY created_at DESC LIMIT $1`)).
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
	uid := uuid.New()
	now := time.Now()

	cols := []string{
		"id", "order_id", "user_id", "transaction_id",
		"transaction_status", "payment_type", "gross_amount",
		"status_code", "fraud_status", "raw_payload",
		"created_at", "updated_at",
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" WHERE id = $1 ORDER BY "payment_transactions"."id" LIMIT $2`)).
		WithArgs(id1.String(), 1).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(id1, "sub-001", uid, "txn-001", "settlement", "gopay", "50000", "200", "accept", "{}", now, now))

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

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payment_transactions" WHERE id = $1 ORDER BY "payment_transactions"."id" LIMIT $2`)).
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
