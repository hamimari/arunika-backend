package handlers

import (
	"bytes"
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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func productColumns() []string {
	return []string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}
}

func TestAdminProductHandler_List_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewProductService(gormDB)
	h := NewAdminProductHandler(svc)

	id1 := uuid.New()
	featureID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" ORDER BY created_at desc`)).
		WillReturnRows(sqlmock.NewRows(productColumns()).
			AddRow(id1, featureID, 29000, true, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "features" WHERE id = $1 ORDER BY "features"."id" LIMIT $2`)).
		WithArgs(featureID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT a.title FROM product_ar_cards pac JOIN ar_cards a ON a.id = pac.ar_card_id WHERE pac.product_id = $1`)).
		WithArgs(id1).
		WillReturnRows(sqlmock.NewRows([]string{"title"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT d.title FROM product_dongengs pd JOIN dongengs d ON d.id = pd.dongeng_id WHERE pd.product_id = $1`)).
		WithArgs(id1).
		WillReturnRows(sqlmock.NewRows([]string{"title"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_ar_cards" WHERE product_id = $1 ORDER BY "product_ar_cards"."product_id" LIMIT $2`)).
		WithArgs(id1, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_dongengs" WHERE product_id = $1 ORDER BY "product_dongengs"."product_id" LIMIT $2`)).
		WithArgs(id1, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/products", nil)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp["data"])
}

func TestAdminProductHandler_List_DBError(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewProductService(gormDB)
	h := NewAdminProductHandler(svc)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" ORDER BY created_at desc`)).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/products", nil)

	h.List(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAdminProductHandler_Get_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewProductService(gormDB)
	h := NewAdminProductHandler(svc)

	id := uuid.New()
	featureID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows(productColumns()).AddRow(id, featureID, 29000, true, now, now))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/products/"+id.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: id.String()}}

	h.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminProductHandler_Get_InvalidID(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewProductService(gormDB)
	h := NewAdminProductHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/products/nope", nil)
	c.Params = gin.Params{{Key: "id", Value: "nope"}}

	h.Get(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminProductHandler_Create_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewProductService(gormDB)
	h := NewAdminProductHandler(svc)

	featureID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "features" WHERE code = $1 ORDER BY "features"."id" LIMIT $2`)).
		WithArgs("AR_CARD", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "is_deleted", "code", "name", "description", "is_active"}).
			AddRow(featureID, now, now, false, "AR_CARD", "AR Card", "desc", true))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "products"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "product_ar_cards"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body, _ := json.Marshal(map[string]interface{}{
		"feature_code": "AR_CARD",
		"price_idr":    29000,
		"ar_card_id":   "card-1",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/products", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminProductHandler_Create_InvalidBody(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	svc := services.NewProductService(gormDB)
	h := NewAdminProductHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/products", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminProductHandler_Update_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewProductService(gormDB)
	h := NewAdminProductHandler(svc)

	id := uuid.New()
	featureID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "products" SET "price_idr"=$1,"updated_at"=$2 WHERE id = $3`)).
		WithArgs(int64(39000), sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows(productColumns()).AddRow(id, featureID, 39000, true, now, now))

	body, _ := json.Marshal(map[string]interface{}{"price_idr": 39000})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/admin/products/"+id.String(), bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id.String()}}

	h.Update(c)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAdminProductHandler_Delete_Conflict(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewProductService(gormDB)
	h := NewAdminProductHandler(svc)

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "product_ar_cards" WHERE product_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "product_dongengs" WHERE product_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "products" WHERE id = $1`)).
		WithArgs(id).
		WillReturnError(&pgconn.PgError{Code: "23503"})
	mock.ExpectRollback()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/admin/products/"+id.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: id.String()}}

	h.Delete(c)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAdminProductHandler_ToggleActive_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	svc := services.NewProductService(gormDB)
	h := NewAdminProductHandler(svc)

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "products" SET "is_active"=$1,"updated_at"=$2 WHERE id = $3`)).
		WithArgs(false, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body, _ := json.Marshal(map[string]interface{}{"is_active": false})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/products/"+id.String()+"/active", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id.String()}}

	h.ToggleActive(c)

	assert.Equal(t, http.StatusOK, w.Code)
}
