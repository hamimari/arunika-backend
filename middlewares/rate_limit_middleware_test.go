package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware_AllowsUnderLimit(t *testing.T) {
	rdb, _ := newRedisClient(t)

	_, engine := gin.CreateTestContext(httptest.NewRecorder())
	engine.Use(RateLimitMiddleware(rdb, "test", 3, time.Minute))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5555"
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestRateLimitMiddleware_BlocksOverLimit(t *testing.T) {
	rdb, _ := newRedisClient(t)

	_, engine := gin.CreateTestContext(httptest.NewRecorder())
	engine.Use(RateLimitMiddleware(rdb, "test", 3, time.Minute))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	var lastCode int
	for i := 0; i < 4; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5555"
		engine.ServeHTTP(w, req)
		lastCode = w.Code
	}

	assert.Equal(t, http.StatusTooManyRequests, lastCode)
}

func TestRateLimitMiddleware_TracksClientsIndependently(t *testing.T) {
	rdb, _ := newRedisClient(t)

	_, engine := gin.CreateTestContext(httptest.NewRecorder())
	engine.Use(RateLimitMiddleware(rdb, "test", 1, time.Minute))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "1.1.1.1:1111"
	engine.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// A different client IP must not be affected by the first client's usage.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "2.2.2.2:2222"
	engine.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestRateLimitMiddleware_FailsOpenWhenRedisIsNil(t *testing.T) {
	_, engine := gin.CreateTestContext(httptest.NewRecorder())
	engine.Use(RateLimitMiddleware(nil, "test", 1, time.Minute))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}
