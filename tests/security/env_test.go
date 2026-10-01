package security_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"arunika_backend/models"
	"arunika_backend/registry"
	"arunika_backend/routes"
	"arunika_backend/tests/fixtures"
)

// The security suite drives the same assembled backend as tests/api — real
// router, real middleware chain, real PostgreSQL — but asserts policy rather
// than features: who is allowed to do what, and what a hostile request gets
// back. It lives in its own package so a failure here reads as "a security
// guarantee broke", not "a feature test failed".

const testSecret = "test-secret-key-at-least-32-chars!!"

var fakePlay *fixtures.FakePlay

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	// Handlers load templates/ by relative path, so run from the repo root the
	// way the server does; otherwise every template-backed route 500s here.
	if err := os.Chdir("../.."); err != nil {
		fmt.Fprintf(os.Stderr, "could not change to repo root: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("JWT_SECRET", testSecret); err != nil {
		fmt.Fprintf(os.Stderr, "could not set JWT_SECRET: %v\n", err)
		os.Exit(1)
	}
	play, err := fixtures.NewFakePlay()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start fake Play server: %v\n", err)
		os.Exit(1)
	}
	fakePlay = play
	for k, v := range map[string]string{
		"GOOGLE_PLAY_SERVICE_ACCOUNT_JSON": play.ServiceAccountJSON,
		"ANDROID_PUBLISHER_BASE_URL":       play.Server.URL,
		"ANDROID_PACKAGE_NAME":             "com.arunika",
	} {
		if err := os.Setenv(k, v); err != nil {
			fmt.Fprintf(os.Stderr, "could not set %s: %v\n", k, err)
			os.Exit(1)
		}
	}

	ctx := context.Background()
	if err := fixtures.StartPostgres(ctx); err != nil {
		if os.Getenv("CI") != "" {
			fmt.Fprintf(os.Stderr, "could not start PostgreSQL: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "skipping security tests — could not start PostgreSQL: %v\n", err)
		os.Exit(0)
	}

	code := m.Run()
	play.Close()
	fixtures.Terminate(ctx)
	os.Exit(code)
}

type Env struct {
	T      *testing.T
	Router *gin.Engine
	DB     *gorm.DB
	Redis  *miniredis.Miniredis
}

func NewEnv(t *testing.T) *Env {
	t.Helper()
	db := fixtures.FreshDB(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	reg := registry.NewServiceRegistry(db, rdb)
	return &Env{T: t, Router: routes.SetupRouter(reg, rdb, db), DB: db, Redis: mr}
}

type Response struct {
	t    *testing.T
	Code int
	Body []byte
}

func (r *Response) JSON() map[string]interface{} {
	r.t.Helper()
	var out map[string]interface{}
	require.NoError(r.t, json.Unmarshal(r.Body, &out), "body was not JSON: %s", string(r.Body))
	return out
}

func (r *Response) Data() map[string]interface{} {
	r.t.Helper()
	data, ok := r.JSON()["data"].(map[string]interface{})
	require.True(r.t, ok, "no data object: %s", string(r.Body))
	return data
}

// Do sends a request with a raw body, so tests can send malformed and
// oversized payloads a JSON encoder would never produce.
func (e *Env) Do(method, path, token string, body []byte) *Response {
	e.T.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.Router.ServeHTTP(w, req)
	return &Response{t: e.T, Code: w.Code, Body: w.Body.Bytes()}
}

func (e *Env) send(method, path, token string, body interface{}) *Response {
	e.T.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		require.NoError(e.T, err)
	}
	return e.Do(method, path, token, raw)
}

func (e *Env) GET(path, token string) *Response { return e.send(http.MethodGet, path, token, nil) }
func (e *Env) POST(path, token string, body interface{}) *Response {
	return e.send(http.MethodPost, path, token, body)
}
func (e *Env) PUT(path, token string, body interface{}) *Response {
	return e.send(http.MethodPut, path, token, body)
}
func (e *Env) PATCH(path, token string, body interface{}) *Response {
	return e.send(http.MethodPatch, path, token, body)
}

type Account struct {
	ID           uuid.UUID
	Email        string
	Password     string
	Token        string
	RefreshToken string
}

// Register signs up through the real endpoint.
func (e *Env) Register(t *testing.T) *Account {
	t.Helper()
	email := fmt.Sprintf("sec-user-%d@example.test", fixtures.NextSeq())
	phone := fmt.Sprintf("0812%08d", fixtures.NextSeq())
	const password = "secret123"

	res := e.POST("/auth/signup", "", map[string]interface{}{
		"name": "Security Test User", "phone_number": phone, "email_address": email,
		"address": "Jl. Test", "city": "Jakarta", "password": password,
		"child": []map[string]string{{"name": "Budi", "gender": "M", "date_of_birth": "2020-01-02T00:00:00.000"}},
	})
	require.Equal(t, http.StatusCreated, res.Code, "signup failed: %s", string(res.Body))

	var user models.Parent
	require.NoError(t, e.DB.Where("email_address = ?", email).First(&user).Error)
	data := res.Data()
	return &Account{
		ID: user.ID, Email: email, Password: password,
		Token: data["token"].(string), RefreshToken: data["refresh_token"].(string),
	}
}

// ChildID returns the id of the account's first child.
func (e *Env) ChildID(t *testing.T, a *Account) uuid.UUID {
	t.Helper()
	var raw string
	require.NoError(t, e.DB.Raw("SELECT id::text FROM children WHERE parent_id = ? LIMIT 1", a.ID).Scan(&raw).Error)
	id, err := uuid.Parse(raw)
	require.NoError(t, err, "account has no child")
	return id
}

// mint signs an access token with arbitrary claims and key, so tests can forge
// exactly the credential an attacker would.
func mint(t *testing.T, method jwt.SigningMethod, key interface{}, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return s
}

func userClaims(sub uuid.UUID, exp time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"sub": sub.String(), "email": "forged@example.test",
		"jti": uuid.NewString(), "exp": exp.Unix(),
	}
}

func adminToken(t *testing.T, sub uuid.UUID, key []byte) string {
	t.Helper()
	claims := userClaims(sub, time.Now().Add(time.Hour))
	claims["role"] = "admin"
	return mint(t, jwt.SigningMethodHS256, key, claims)
}

func urlEscape(s string) string { return url.QueryEscape(s) }
