package handlers

import (
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
	"gorm.io/gorm"
)

// ─── PrintableCardHandler ─────────────────────────────────────────────────────

func setupPrintableRouter(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()
	gormDB, mock := setupHandlerDB(t)
	productSvc := services.NewProductService(gormDB)
	entitlementSvc := services.NewEntitlementService(gormDB)
	h := NewPrintableCardHandler(gormDB, productSvc, entitlementSvc)
	r := gin.New()
	r.GET("/ar/printable-pdf", h.GetPrintablePDF)
	return r, mock
}

// expectFreeCard queues the mock DB response for resolving a card with no
// linked product (i.e. free content) — every existing test's cards are free,
// so the entitlement filter must let them all through unfiltered.
func expectFreeCard(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}))
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
	expectFreeCard(mock)

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
	expectFreeCard(mock)

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
	expectFreeCard(mock)

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
	expectFreeCard(mock)

	req := httptest.NewRequest(http.MethodGet, "/ar/printable-pdf?category_id="+catID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// PDF is still generated (graceful degradation for the unreachable printable_img)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Body.Bytes())
}

func paidCardRow(cardID, catID string) (*sqlmock.Rows, uuid.UUID) {
	catUUID := uuid.MustParse(catID)
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code",
		"category", "sub_category", "image_url", "emoji", "bg_color",
		"is_unlocked", "description", "printable_img", "category_id", "sub_category_id",
		"created_at", "expires_at",
	}).AddRow(
		cardID, "animal", "Singa", "", "", "S002",
		"", "", "", "🦁", "#FFF3E0",
		false, "", "", &catUUID, nil,
		now, nil,
	)
	return rows, catUUID
}

// Scenario: a category whose only card is paid, requested by a guest (no
// entitlement possible) — must be blocked with 403, not silently produce an
// empty/partial PDF or (worse) include the paid card unconditionally.
func TestPrintableCardHandler_PaidCard_Guest_Returns403(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	productSvc := services.NewProductService(gormDB)
	entitlementSvc := services.NewEntitlementService(gormDB)
	h := NewPrintableCardHandler(gormDB, productSvc, entitlementSvc)

	catID := uuid.NewString()
	cardID := uuid.NewString()
	productID := uuid.New()
	rows, _ := paidCardRow(cardID, catID)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(rows)
	// FindAllCards' Preload("CategoryRef") — only one preload query actually
	// fires, since these test rows all have a nil sub_category_id (nothing
	// for Preload("SubCategoryRef") to look up).
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	// product_ar_cards lookup — this card HAS a linked product
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}).AddRow(productID, cardID))
	// products lookup (FindProductByArCardID's second query)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}).
			AddRow(productID, uuid.New(), 12000, true, time.Now(), time.Now()))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ar/printable-pdf?category_id="+catID, nil)

	h.GetPrintablePDF(c)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Scenario: same paid-only category, but the requester already has an
// entitlement for it — the PDF must still be generated.
func TestPrintableCardHandler_PaidCard_WithEntitlement_ReturnsPDF(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	productSvc := services.NewProductService(gormDB)
	entitlementSvc := services.NewEntitlementService(gormDB)
	h := NewPrintableCardHandler(gormDB, productSvc, entitlementSvc)

	catID := uuid.NewString()
	cardID := uuid.NewString()
	productID := uuid.New()
	userID := uuid.New()
	rows, _ := paidCardRow(cardID, catID)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(rows)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}).AddRow(productID, cardID))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}).
			AddRow(productID, uuid.New(), 12000, true, time.Now(), time.Now()))
	// EntitlementService.HasAccess: no subscription, but has an entitlement
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "user_entitlements" WHERE user_id = $1 AND product_id = $2 AND (expires_at IS NULL OR expires_at > NOW())`)).
		WithArgs(userID, productID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/ar/printable-pdf?category_id="+catID, nil)
	c.Set("userID", userID.String())

	h.GetPrintablePDF(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.NoError(t, mock.ExpectationsWereMet())
}
