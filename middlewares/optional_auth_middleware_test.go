package middlewares

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestOptionalAuthMiddleware_NoHeader_ProceedsUnauthenticated(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, _ := newRedisClient(t)
	var sawUserID bool
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(OptionalAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) {
		_, sawUserID = c.Get("userID")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, sawUserID)
}

func TestOptionalAuthMiddleware_InvalidToken_ProceedsUnauthenticated(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, _ := newRedisClient(t)
	var sawUserID bool
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(OptionalAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) {
		_, sawUserID = c.Get("userID")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.token")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, sawUserID)
}

func TestOptionalAuthMiddleware_ExpiredToken_ProceedsUnauthenticated(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, _ := newRedisClient(t)
	tokenStr := makeToken(t, testSecret, -1*time.Hour)

	var sawUserID bool
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(OptionalAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) {
		_, sawUserID = c.Get("userID")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, sawUserID)
}

func TestOptionalAuthMiddleware_ValidToken_SetsUserID(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, _ := newRedisClient(t)
	tokenStr := makeToken(t, testSecret, 15*time.Minute)

	var gotUserID interface{}
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(OptionalAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) {
		gotUserID, _ = c.Get("userID")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user-1", gotUserID)
}

func TestOptionalAuthMiddleware_RevokedToken_ProceedsUnauthenticated(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, mr := newRedisClient(t)

	claims := jwt.MapClaims{
		"sub": "user-1",
		"jti": "test-jti-revoked",
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := tok.SignedString([]byte(testSecret))
	assert.NoError(t, err)
	mr.Set("blacklist:test-jti-revoked", "revoked")

	var sawUserID bool
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(OptionalAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) {
		_, sawUserID = c.Get("userID")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, sawUserID)
}
