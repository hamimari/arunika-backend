package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── SecurityHeadersMiddleware ────────────────────────────────────────────────

func TestSecurityHeadersMiddleware_SetsHeadersOnEverySuccessfulResponse(t *testing.T) {
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

// Headers that only appear on success are worthless — an error page is just
// as embeddable and just as sniffable.
func TestSecurityHeadersMiddleware_SetsHeadersOnErrorResponsesToo(t *testing.T) {
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/boom", func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "nope"})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestSecurityHeadersMiddleware_SetsHeadersOn404(t *testing.T) {
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nothing-here", nil))

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
}

// ─── ErrorMiddleware ──────────────────────────────────────────────────────────

func TestErrorMiddleware_RecoversFromPanicWithoutLeakingDetail(t *testing.T) {
	r := gin.New()
	r.Use(ErrorMiddleware())
	r.GET("/panic", func(c *gin.Context) {
		panic("connection string postgres://user:hunter2@db:5432 failed")
	})

	w := httptest.NewRecorder()
	// The panic must not escape and crash the test binary either.
	require.NotPanics(t, func() {
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"internal server error"}`, w.Body.String())

	// Whatever was in the panic value stays server-side.
	assert.NotContains(t, w.Body.String(), "hunter2")
	assert.NotContains(t, w.Body.String(), "postgres://")
	assert.NotContains(t, w.Body.String(), "goroutine")
}

func TestErrorMiddleware_PanicWithAnErrorValue_IsAlsoRecovered(t *testing.T) {
	r := gin.New()
	r.Use(ErrorMiddleware())
	r.GET("/panic", func(c *gin.Context) { panic(assertAnError{}) })

	w := httptest.NewRecorder()
	require.NotPanics(t, func() {
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

type assertAnError struct{}

func (assertAnError) Error() string { return "boom" }

func TestErrorMiddleware_HappyPathIsUntouched(t *testing.T) {
	r := gin.New()
	r.Use(ErrorMiddleware())
	r.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"value": 42}) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"value":42}`, w.Body.String(),
		"a handler that does not panic must pass through unchanged")
}

// A panic in one request must not poison the next: the recovery is
// per-request, and the router must keep serving.
func TestErrorMiddleware_RouterKeepsServingAfterAPanic(t *testing.T) {
	r := gin.New()
	r.Use(ErrorMiddleware())
	r.GET("/panic", func(c *gin.Context) { panic("first") })
	r.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	first := httptest.NewRecorder()
	r.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/panic", nil))
	require.Equal(t, http.StatusInternalServerError, first.Code)

	second := httptest.NewRecorder()
	r.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/ok", nil))
	assert.Equal(t, http.StatusOK, second.Code)
}
