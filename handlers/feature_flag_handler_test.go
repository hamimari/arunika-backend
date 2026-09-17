package handlers

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"arunika_backend/services"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestFeatureFlagHandler_GetPublic(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewFeatureFlagHandler(services.NewFeatureFlagService(gormDB))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "app_feature_flags" ORDER BY key ASC`)).
		WillReturnRows(sqlmock.NewRows([]string{"key", "name", "description", "is_enabled", "updated_at"}).
			AddRow("printable_cards", "Kartu Printable", "", false, time.Now()).
			AddRow("qr_scan", "Scan QR", "", true, time.Now()))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/app/feature-flags", nil)
	h.GetPublic(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"data":{"printable_cards":false,"qr_scan":true}}`, w.Body.String())
}

func TestFeatureFlagHandler_AdminToggle_MissingBody(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	h := NewFeatureFlagHandler(services.NewFeatureFlagService(gormDB))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/feature-flags/qr_scan", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "key", Value: "qr_scan"}}
	h.AdminToggle(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFeatureFlagHandler_AdminToggle_UnknownKey(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewFeatureFlagHandler(services.NewFeatureFlagService(gormDB))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "app_feature_flags"`)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/feature-flags/nope", strings.NewReader(`{"is_enabled":false}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "key", Value: "nope"}}
	h.AdminToggle(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
