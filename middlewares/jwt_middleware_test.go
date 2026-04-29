package middlewares

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const testSecret = "test-secret-key-at-least-32-chars!!"

func makeToken(t *testing.T, secret string, expOffset time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":           "user-1",
		"email":         "u@example.com",
		"refresh_token": uuid.NewString(),
		"jti":           uuid.NewString(),
		"exp":           time.Now().Add(expOffset).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func newRedisClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return rdb, mr
}

func TestJWTMiddleware_MissingAuthHeader(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, _ := newRedisClient(t)
	w := httptest.NewRecorder()
	c, engine := gin.CreateTestContext(w)
	engine.Use(JWTAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request = req
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, _ := newRedisClient(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(JWTAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.token")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, _ := newRedisClient(t)
	tokenStr := makeToken(t, testSecret, -1*time.Hour)

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(JWTAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, _ := newRedisClient(t)
	tokenStr := makeToken(t, testSecret, 15*time.Minute)

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(JWTAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJWTMiddleware_RevokedToken(t *testing.T) {
	os.Setenv("JWT_SECRET", testSecret)
	defer os.Unsetenv("JWT_SECRET")

	rdb, mr := newRedisClient(t)

	// Build a valid token and extract its jti
	claims := jwt.MapClaims{
		"sub":           "user-1",
		"email":         "u@example.com",
		"refresh_token": uuid.NewString(),
		"jti":           "test-jti-blacklisted",
		"exp":           time.Now().Add(15 * time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)

	// Put the jti in the blacklist
	mr.Set("blacklist:test-jti-blacklisted", "revoked")

	_ = rdb
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(JWTAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTMiddleware_MissingSecret(t *testing.T) {
	os.Unsetenv("JWT_SECRET")

	rdb, _ := newRedisClient(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(JWTAuthMiddleware(rdb))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
