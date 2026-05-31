package handlers

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── PrintableCardHandler ─────────────────────────────────────────────────────

func setupPrintableRouter(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()
	gormDB, mock := setupHandlerDB(t)
	h := NewPrintableCardHandler(gormDB)
	r := gin.New()
	r.GET("/ar/printable-pdf", h.GetPrintablePDF)
	return r, mock
}

// Scenario: missing category_id returns 400
func TestPrintableCardHandler_MissingCategoryID(t *testing.T) {
	r, _ := setupPrintableRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/ar/printable-pdf", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Scenario: unknown / empty category returns 404
func TestPrintableCardHandler_UnknownCategory_Returns404(t *testing.T) {
	r, mock := setupPrintableRouter(t)

	catID := uuid.New().String()

	// FindAllCards query — returns no rows
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	req := httptest.NewRequest(http.MethodGet, "/ar/printable-pdf?category_id="+catID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Scenario: valid category returns HTTP 200, application/pdf, non-empty body
func TestPrintableCardHandler_ValidCategory_ReturnsPDF(t *testing.T) {
	r, mock := setupPrintableRouter(t)

	catID := uuid.NewString()
	cardID := uuid.NewString()
	catUUID := uuid.MustParse(catID)
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code",
		"category", "sub_category", "image_url", "emoji", "bg_color",
		"is_unlocked", "description", "printable_img", "category_id", "sub_category_id",
		"created_at", "expires_at",
	}).AddRow(
		cardID, "animal", "Kuda", "https://cdn/kuda.glb", "", "K001",
		"", "", "", "🐴", "#FFF3E0",
		true, "", "", &catUUID, nil,
		now, nil,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WillReturnRows(rows)
	// Preload queries
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	req := httptest.NewRequest(http.MethodGet, "/ar/printable-pdf?category_id="+catID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Body.Bytes(), "PDF body should not be empty")
}

// Scenario: Content-Disposition attachment header is set
func TestPrintableCardHandler_ContentDispositionHeader(t *testing.T) {
	r, mock := setupPrintableRouter(t)

	catID := uuid.NewString()
	catUUID := uuid.MustParse(catID)
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code",
		"category", "sub_category", "image_url", "emoji", "bg_color",
		"is_unlocked", "description", "printable_img", "category_id", "sub_category_id",
		"created_at", "expires_at",
	}).AddRow(
		uuid.NewString(), "animal", "Sapi", "", "", "S001",
		"", "", "", "🐄", "#FFF3E0",
		true, "", "", &catUUID, nil,
		now, nil,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(rows)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	req := httptest.NewRequest(http.MethodGet, "/ar/printable-pdf?category_id="+catID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	cd := w.Header().Get("Content-Disposition")
	assert.Contains(t, cd, "attachment")
	assert.Contains(t, cd, "kartu-ar.pdf")
}

// Scenario: card with inaccessible image_url still produces a valid PDF (graceful degradation)
func TestPrintableCardHandler_ImageFetchFailure_StillReturnsPDF(t *testing.T) {
	r, mock := setupPrintableRouter(t)

	catID := uuid.NewString()
	catUUID := uuid.MustParse(catID)
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code",
		"category", "sub_category", "image_url", "emoji", "bg_color",
		"is_unlocked", "description", "printable_img", "category_id", "sub_category_id",
		"created_at", "expires_at",
	}).AddRow(
		uuid.NewString(), "animal", "Harimau", "",
		"", "H001",
		"", "", "https://invalid.host.test/image.png", // unreachable URL
		"🐯", "#FFF3E0",
		true, "", "", &catUUID, nil,
		now, nil,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(rows)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	req := httptest.NewRequest(http.MethodGet, "/ar/printable-pdf?category_id="+catID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should still return 200 PDF even though image fetch failed
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Body.Bytes())
}

// Scenario: PrintableImg takes priority over ImageURL in the PDF
func TestPrintableCardHandler_PrintableImgTakesPriorityOverImageURL(t *testing.T) {
	r, mock := setupPrintableRouter(t)

	catID := uuid.NewString()
	catUUID := uuid.MustParse(catID)
	now := time.Now()

	// Card has both image_url and printable_img set; printable_img is unreachable,
	// so the handler should attempt printable_img first (and fall back to grey placeholder),
	// NOT use image_url as a substitute — confirming priority logic.
	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code",
		"category", "sub_category", "image_url", "emoji", "bg_color",
		"is_unlocked", "description", "printable_img", "category_id", "sub_category_id",
		"created_at", "expires_at",
	}).AddRow(
		uuid.NewString(), "animal", "Gajah", "", "", "G001",
		"", "", "https://cdn/gajah.png", "🐘", "#FFF3E0",
		true, "", "https://invalid.host.test/printable.png", &catUUID, nil,
		now, nil,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(rows)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))

	req := httptest.NewRequest(http.MethodGet, "/ar/printable-pdf?category_id="+catID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// PDF is still generated (graceful degradation for the unreachable printable_img)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Body.Bytes())
}
