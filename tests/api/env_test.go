package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"arunika_backend/registry"
	"arunika_backend/routes"
	"arunika_backend/tests/fixtures"
)

const testSecret = "test-secret-key-at-least-32-chars!!"

// One fake Android Publisher for the whole package. Tests register their own
// uniquely-named purchase tokens against it, so it is safe to share across
// parallel tests — and the environment variables pointing the verifier at it
// are set once in TestMain, where they cannot clash with t.Parallel.
var fakePlay *fixtures.FakePlay

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	// Process-wide rather than per-test: t.Setenv is incompatible with
	// t.Parallel, and the secret is constant for the whole package anyway.
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
		fmt.Fprintf(os.Stderr, "skipping API tests — could not start PostgreSQL: %v\n", err)
		os.Exit(0)
	}

	code := m.Run()
	play.Close()
	fixtures.Terminate(ctx)
	os.Exit(code)
}

// Env is the whole backend, assembled the way main.go assembles it: the real
// route table, the real middleware chain, a real PostgreSQL database and a
// Redis substitute. Tests drive it over HTTP, so a request passes through
// every layer it would in production.
//
// This is what handler tests with sqlmock cannot do — they call a handler
// function directly, so the middleware chain, route binding and real SQL are
// all absent from what they prove.
type Env struct {
	T      *testing.T
	Router *gin.Engine
	DB     *gorm.DB
	Redis  *miniredis.Miniredis
}

func NewAPIEnv(t *testing.T) *Env {
	t.Helper()
	db := fixtures.FreshDB(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	reg := registry.NewServiceRegistry(db, rdb)
	return &Env{T: t, Router: routes.SetupRouter(reg, rdb, db), DB: db, Redis: mr}
}

// Response wraps an HTTP result with assertions tests actually want to make.
type Response struct {
	t    *testing.T
	Code int
	Body []byte
}

func (r *Response) JSON() map[string]interface{} {
	r.t.Helper()
	var out map[string]interface{}
	require.NoError(r.t, json.Unmarshal(r.Body, &out),
		"response body was not JSON: %s", string(r.Body))
	return out
}

// Data returns the "data" envelope most endpoints wrap their payload in.
func (r *Response) Data() map[string]interface{} {
	r.t.Helper()
	data, ok := r.JSON()["data"].(map[string]interface{})
	require.True(r.t, ok, "response has no data object: %s", string(r.Body))
	return data
}

func (e *Env) do(method, path, token string, body interface{}) *Response {
	e.T.Helper()

	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(e.T, err)
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	e.Router.ServeHTTP(w, req)
	return &Response{t: e.T, Code: w.Code, Body: w.Body.Bytes()}
}

func (e *Env) GET(path, token string) *Response    { return e.do(http.MethodGet, path, token, nil) }
func (e *Env) DELETE(path, token string) *Response { return e.do(http.MethodDelete, path, token, nil) }

func (e *Env) POST(path, token string, body interface{}) *Response {
	return e.do(http.MethodPost, path, token, body)
}

func (e *Env) PUT(path, token string, body interface{}) *Response {
	return e.do(http.MethodPut, path, token, body)
}

// Account is a registered user plus the credentials to act as them.
type Account struct {
	ID           string
	Email        string
	Password     string
	Token        string
	RefreshToken string
}

// Register signs a new user up through the real endpoint, so the account is
// created exactly as a real one is — including the email-verification
// dispatch and session issuance.
func (e *Env) Register(t *testing.T) *Account {
	t.Helper()
	email := fmt.Sprintf("api-user-%d@example.test", fixtures.NextSeq())
	phone := fmt.Sprintf("0812%08d", fixtures.NextSeq())
	const password = "secret123"

	res := e.POST("/auth/signup", "", map[string]interface{}{
		"name":          "API Test User",
		"phone_number":  phone,
		"email_address": email,
		"address":       "Jl. Test",
		"city":          "Jakarta",
		"password":      password,
		"child": []map[string]string{
			{"name": "Budi", "gender": "M", "date_of_birth": "2020-01-02T00:00:00.000"},
		},
	})
	require.Equal(t, http.StatusCreated, res.Code, "signup failed: %s", string(res.Body))

	data := res.Data()
	return &Account{
		ID:           data["id"].(string),
		Email:        email,
		Password:     password,
		Token:        data["token"].(string),
		RefreshToken: data["refresh_token"].(string),
	}
}

// Login exchanges credentials for a fresh token pair.
func (e *Env) Login(t *testing.T, email, password string) *Response {
	t.Helper()
	return e.POST("/auth/login", "", map[string]string{"email": email, "password": password})
}
