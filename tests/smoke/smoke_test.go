//go:build smoke

// Package smoke is the read-only check run against a deployed environment:
// staging after a deploy, and production after a promote. It seeds nothing,
// buys nothing and mutates nothing — the only credentials it uses are a
// dedicated canary account that already exists there.
//
//	SMOKE_BASE_URL=https://api.example.com \
//	SMOKE_CANARY_EMAIL=... SMOKE_CANARY_PASSWORD=... \
//	go test -tags smoke ./tests/smoke/ -count=1 -v
//
// The canary tests are skipped, not failed, when no canary is configured, so
// the anonymous checks can still run in an environment without one.
package smoke

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var client = &http.Client{Timeout: 15 * time.Second}

func baseURL(t *testing.T) string {
	t.Helper()
	u := strings.TrimRight(os.Getenv("SMOKE_BASE_URL"), "/")
	if u == "" {
		t.Skip("SMOKE_BASE_URL not set")
	}
	return u
}

func do(t *testing.T, method, path, token string, body any) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, baseURL(t)+path, reader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	require.NoError(t, err, "%s %s", method, path)
	defer res.Body.Close()
	out, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, out
}

func TestSmoke_Health(t *testing.T) {
	code, body := do(t, http.MethodGet, "/health", "", nil)
	assert.Equal(t, http.StatusOK, code, string(body))
}

func TestSmoke_AnonymousContentList(t *testing.T) {
	for _, path := range []string{"/fairy-tales", "/ar/cards", "/dongeng-categories", "/banners", "/app/feature-flags"} {
		t.Run(path, func(t *testing.T) {
			code, body := do(t, http.MethodGet, path, "", nil)
			assert.Equal(t, http.StatusOK, code, string(body))
			assert.True(t, json.Valid(body), "not JSON: %.200s", string(body))
		})
	}
}

func TestSmoke_ProtectedRouteRejectsAnonymous(t *testing.T) {
	code, _ := do(t, http.MethodGet, "/orders", "", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestSmoke_AdminRouteRejectsAnonymous(t *testing.T) {
	code, _ := do(t, http.MethodGet, "/admin/users", "", nil)
	assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, code)
}

// login signs the canary in and returns its access token, then signs it out
// so smoke runs do not leave live sessions behind.
func login(t *testing.T) string {
	t.Helper()
	email, password := os.Getenv("SMOKE_CANARY_EMAIL"), os.Getenv("SMOKE_CANARY_PASSWORD")
	if email == "" || password == "" {
		t.Skip("SMOKE_CANARY_EMAIL / SMOKE_CANARY_PASSWORD not set")
	}
	code, body := do(t, http.MethodPost, "/auth/login", "", map[string]string{"email": email, "password": password})
	require.Equal(t, http.StatusOK, code, "canary login failed: %s", string(body))
	var out struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	require.NotEmpty(t, out.AccessToken)
	t.Cleanup(func() { do(t, http.MethodPost, "/auth/logout", out.AccessToken, nil) })
	return out.AccessToken
}

func TestSmoke_CanaryLoginAndOrders(t *testing.T) {
	token := login(t)

	code, body := do(t, http.MethodGet, "/orders", token, nil)

	assert.Equal(t, http.StatusOK, code, string(body))
	assert.True(t, json.Valid(body), "not JSON: %.200s", string(body))
}
