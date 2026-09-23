package middlewares

import (
	"net/http"
	"net/http/httptest"
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

// AdminAuthMiddleware is the boundary between the admin backoffice and the
// mobile app. Everything under /admin sits behind it, so a hole here exposes
// user management, content and payment data to any signed-in app user.

func adminToken(t *testing.T, secret, role string, expOffset time.Duration) (string, string) {
	t.Helper()
	jti := uuid.NewString()
	claims := jwt.MapClaims{
		"sub":           uuid.NewString(),
		"email":         "admin@example.test",
		"refresh_token": uuid.NewString(),
		"jti":           jti,
		"exp":           time.Now().Add(expOffset).Unix(),
	}
	if role != "" {
		claims["role"] = role
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return signed, jti
}

func newAdminRouter(t *testing.T) (*gin.Engine, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	r := gin.New()
	r.GET("/admin/thing", AdminAuthMiddleware(rdb), func(c *gin.Context) {
		adminID, _ := c.Get("adminID")
		c.JSON(http.StatusOK, gin.H{"admin_id": adminID})
	})
	return r, mr
}

func callAdmin(r *gin.Engine, bearer string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/thing", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestAdminAuthMiddleware_ValidAdminToken_IsAllowed(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	r, _ := newAdminRouter(t)

	token, _ := adminToken(t, testSecret, "admin", time.Hour)

	w := callAdmin(r, token)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "admin_id", "downstream handlers need adminID on the context")
}

// The escalation case: a perfectly valid *user* token must not open admin
// routes. Mobile tokens are signed with the same secret, so the role claim is
// the only thing standing between an app user and the backoffice.
func TestAdminAuthMiddleware_ValidUserToken_IsForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	r, _ := newAdminRouter(t)

	for _, role := range []string{"", "user"} {
		name := "role=" + role
		if role == "" {
			name = "role absent"
		}
		t.Run(name, func(t *testing.T) {
			token, _ := adminToken(t, testSecret, role, time.Hour)

			w := callAdmin(r, token)

			assert.Equal(t, http.StatusForbidden, w.Code,
				"a mobile user token must never reach an admin route")
		})
	}
}

func TestAdminAuthMiddleware_MissingHeader_IsUnauthorized(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	r, _ := newAdminRouter(t)

	assert.Equal(t, http.StatusUnauthorized, callAdmin(r, "").Code)
}

func TestAdminAuthMiddleware_ExpiredToken_IsUnauthorized(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	r, _ := newAdminRouter(t)

	token, _ := adminToken(t, testSecret, "admin", -time.Hour)

	assert.Equal(t, http.StatusUnauthorized, callAdmin(r, token).Code)
}

// Signed with a different key: the signature must be checked, not just the
// claims decoded.
func TestAdminAuthMiddleware_WrongSigningKey_IsUnauthorized(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	r, _ := newAdminRouter(t)

	token, _ := adminToken(t, "a-completely-different-secret-key!!", "admin", time.Hour)

	assert.Equal(t, http.StatusUnauthorized, callAdmin(r, token).Code)
}

func TestAdminAuthMiddleware_MalformedToken_IsUnauthorized(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	r, _ := newAdminRouter(t)

	assert.Equal(t, http.StatusUnauthorized, callAdmin(r, "not.a.jwt").Code)
}

// Logging out blacklists the token's jti in Redis; reusing it afterwards must
// fail even though the token itself is still cryptographically valid and
// unexpired.
func TestAdminAuthMiddleware_RevokedToken_IsUnauthorized(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	r, mr := newAdminRouter(t)

	token, jti := adminToken(t, testSecret, "admin", time.Hour)
	require.Equal(t, http.StatusOK, callAdmin(r, token).Code, "sanity: valid before logout")

	require.NoError(t, mr.Set("blacklist:"+jti, "revoked"))

	assert.Equal(t, http.StatusUnauthorized, callAdmin(r, token).Code,
		"a logged-out admin token must stop working")
}

func TestAdminAuthMiddleware_MissingSecret_IsServerError(t *testing.T) {
	// Empty rather than unset: os.Getenv cannot distinguish them, and the
	// middleware treats both as misconfiguration.
	t.Setenv("JWT_SECRET", "")
	r, _ := newAdminRouter(t)

	token, _ := adminToken(t, testSecret, "admin", time.Hour)

	assert.Equal(t, http.StatusInternalServerError, callAdmin(r, token).Code,
		"a missing secret is a server fault, not a client one")
}
