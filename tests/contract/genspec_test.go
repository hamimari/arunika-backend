package contract_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"arunika_backend/registry"
	"arunika_backend/routes"
	"arunika_backend/tests/fixtures"
)

// Run with -run TestGenerateSecurityMap to refresh /tmp/security.json.
func TestGenerateSecurityMap(t *testing.T) {
	if os.Getenv("GENERATE_SPEC") == "" {
		t.Skip("set GENERATE_SPEC=1 to regenerate")
	}
	gin.SetMode(gin.TestMode)
	require.NoError(t, os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars!!"))

	db := fixtures.FreshDB(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	reg := registry.NewServiceRegistry(db, rdb)
	r := routes.SetupRouter(reg, rdb, db)

	userToken := mint(t, "user")

	type entry struct{ Method, Path, Security string }
	var out []entry
	for _, ri := range r.Routes() {
		path := concretise(ri.Path)
		anon := probe(r, ri.Method, path, "")
		sec := "none"
		if anon == http.StatusUnauthorized {
			sec = "bearerAuth"
			if probe(r, ri.Method, path, userToken) == http.StatusForbidden {
				sec = "adminAuth"
			}
		}
		out = append(out, entry{ri.Method, ri.Path, sec})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	b, _ := json.MarshalIndent(out, "", "  ")
	require.NoError(t, os.WriteFile("/tmp/security.json", b, 0o644))
	t.Logf("wrote %d routes", len(out))
}

func mint(t *testing.T, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub": uuid.NewString(), "email": "probe@example.test",
		"refresh_token": uuid.NewString(), "jti": uuid.NewString(),
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	if role != "" {
		claims["role"] = role
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte("test-secret-key-at-least-32-chars!!"))
	require.NoError(t, err)
	return s
}

// concretise replaces :params with a placeholder so routing matches.
func concretise(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		if strings.HasPrefix(seg, ":") || strings.HasPrefix(seg, "*") {
			parts[i] = "00000000-0000-0000-0000-000000000000"
		}
	}
	return strings.Join(parts, "/")
}

func probe(r *gin.Engine, method, path, token string) int {
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}
