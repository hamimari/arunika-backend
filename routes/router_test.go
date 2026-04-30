package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"arunika_backend/registry"
)

func setupRouterDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	dialector := postgres.New(postgres.Config{Conn: db, DriverName: "postgres"})
	gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return gormDB, mock
}

// TestRoutes_AdminEndpointsRegistered verifies that all admin routes return
// something other than 404, proving they are registered in the router.
// The requests will return 401 (missing/invalid auth) — that's expected and correct.
func TestRoutes_AdminEndpointsRegistered(t *testing.T) {
	db, _ := setupRouterDB(t)
	rdb, _ := redismock.NewClientMock()
	reg := registry.NewServiceRegistry(db, rdb)
	r := SetupRouter(reg, rdb)

	routes := []struct {
		method string
		path   string
	}{
		// Auth
		{http.MethodPost, "/admin/auth/login"},
		{http.MethodPost, "/admin/auth/refresh"},
		{http.MethodPost, "/admin/auth/logout"},
		// Analytics
		{http.MethodGet, "/admin/analytics/dau"},
		{http.MethodGet, "/admin/analytics/new-users"},
		{http.MethodGet, "/admin/analytics/popular-features"},
		{http.MethodGet, "/admin/analytics/payments"},
		{http.MethodGet, "/admin/analytics/subscription-stats"},
		// Payments
		{http.MethodGet, "/admin/payments"},
		{http.MethodGet, "/admin/payments/some-id"},
		// Users
		{http.MethodGet, "/admin/users"},
		{http.MethodGet, "/admin/users/some-id"},
		{http.MethodPatch, "/admin/users/some-id/permission"},
		// Campaigns
		{http.MethodPost, "/admin/campaigns"},
		// Banners (admin)
		{http.MethodGet, "/admin/content/banners"},
		{http.MethodPost, "/admin/content/banners"},
		{http.MethodGet, "/admin/content/banners/some-id"},
		{http.MethodPut, "/admin/content/banners/some-id"},
		{http.MethodDelete, "/admin/content/banners/some-id"},
		{http.MethodPatch, "/admin/content/banners/some-id/visibility"},
		{http.MethodPatch, "/admin/content/banners/some-id/active"},
		// Content — Fairy Tales
		{http.MethodGet, "/admin/content/fairy-tales"},
		{http.MethodPost, "/admin/content/fairy-tales"},
		{http.MethodGet, "/admin/content/fairy-tales/some-id"},
		{http.MethodPut, "/admin/content/fairy-tales/some-id"},
		{http.MethodDelete, "/admin/content/fairy-tales/some-id"},
		{http.MethodPatch, "/admin/content/fairy-tales/some-id/visibility"},
		// Content — AR Cards
		{http.MethodGet, "/admin/content/ar-cards"},
		{http.MethodPost, "/admin/content/ar-cards"},
		// Content — Tracing Items
		{http.MethodGet, "/admin/content/tracing-items"},
		{http.MethodPost, "/admin/content/tracing-items"},
		// Content — Counting Questions
		{http.MethodGet, "/admin/content/counting-questions"},
		{http.MethodPost, "/admin/content/counting-questions"},
		// Content — Badges
		{http.MethodGet, "/admin/content/badges"},
		{http.MethodPost, "/admin/content/badges"},
		// Content — Categories
		{http.MethodGet, "/admin/content/categories"},
		{http.MethodPost, "/admin/content/categories"},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			r.ServeHTTP(w, req)
			// 404 means the route is NOT registered — any other status is acceptable.
			assert.NotEqual(t, http.StatusNotFound, w.Code,
				"route %s %s returned 404 — it is not registered in the router", tc.method, tc.path)
		})
	}
}

// TestRoutes_PublicBannersEndpoint verifies the mobile-facing banner endpoint.
func TestRoutes_PublicBannersEndpoint(t *testing.T) {
	db, _ := setupRouterDB(t)
	rdb, _ := redismock.NewClientMock()
	reg := registry.NewServiceRegistry(db, rdb)
	r := SetupRouter(reg, rdb)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/banners", nil)
	r.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusNotFound, w.Code)
}
