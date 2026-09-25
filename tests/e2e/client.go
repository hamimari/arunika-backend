//go:build e2e

package e2e

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

var seq atomic.Int64

func nextSeq() int64 { return seq.Add(1) }

// Resp is an HTTP result with the extractors flows actually need.
type Resp struct {
	t    *testing.T
	Code int
	Body []byte
}

func (r *Resp) JSON() map[string]interface{} {
	r.t.Helper()
	var out map[string]interface{}
	require.NoError(r.t, json.Unmarshal(r.Body, &out), "not JSON: %s", string(r.Body))
	return out
}

// Data returns the "data" object most endpoints wrap their payload in.
func (r *Resp) Data() map[string]interface{} {
	r.t.Helper()
	d, ok := r.JSON()["data"].(map[string]interface{})
	require.True(r.t, ok, "no data object: %s", string(r.Body))
	return d
}

// List returns the "data" array list endpoints return.
func (r *Resp) List() []map[string]interface{} {
	r.t.Helper()
	raw, ok := r.JSON()["data"].([]interface{})
	require.True(r.t, ok, "no data array: %s", string(r.Body))
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		require.True(r.t, ok, "list item is not an object: %v", item)
		out = append(out, m)
	}
	return out
}

func (s *Stack) do(method, path, token string, body interface{}) *Resp {
	s.t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		require.NoError(s.t, err)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, s.BaseURL+path, reader)
	require.NoError(s.t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := s.http.Do(req)
	require.NoError(s.t, err)
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	require.NoError(s.t, err)
	return &Resp{t: s.t, Code: res.StatusCode, Body: b}
}

func (s *Stack) GET(path, token string) *Resp { return s.do(http.MethodGet, path, token, nil) }
func (s *Stack) POST(path, token string, body interface{}) *Resp {
	return s.do(http.MethodPost, path, token, body)
}
func (s *Stack) PATCH(path, token string, body interface{}) *Resp {
	return s.do(http.MethodPatch, path, token, body)
}

// DB opens a direct connection for asserting on state no endpoint exposes
// (entitlement row counts, payment rows). Flows still act only through HTTP.
func (s *Stack) DB() *sql.DB {
	s.t.Helper()
	db, err := sql.Open("pgx", postgresDSN)
	require.NoError(s.t, err)
	s.t.Cleanup(func() { _ = db.Close() })
	return db
}

func (s *Stack) Count(query string, args ...interface{}) int {
	s.t.Helper()
	var n int
	require.NoError(s.t, s.DB().QueryRow(query, args...).Scan(&n))
	return n
}

// Session is an authenticated app user.
type Session struct {
	Email  string
	Token  string
	UserID string
}

// sessionFrom extracts tokens from either response shape the auth endpoints
// use (top-level access_token/user_id on login; a data envelope on signup).
func sessionFrom(t *testing.T, email string, res *Resp) *Session {
	t.Helper()
	body := res.JSON()
	if data, ok := body["data"].(map[string]interface{}); ok {
		body = data
	}
	tok, _ := body["access_token"].(string)
	if tok == "" {
		tok, _ = body["token"].(string)
	}
	require.NotEmpty(t, tok, "no token in response: %s", string(res.Body))
	uid, _ := body["user_id"].(string)
	if uid == "" {
		uid = userIDFromToken(t, tok)
	}
	return &Session{Email: email, Token: tok, UserID: uid}
}

// Login signs in through the real endpoint.
func (s *Stack) Login(email, password string) *Session {
	s.t.Helper()
	res := s.POST("/auth/login", "", map[string]string{"email": email, "password": password})
	require.Equal(s.t, http.StatusOK, res.Code, "login %s: %s", email, string(res.Body))
	return sessionFrom(s.t, email, res)
}

// Signup registers a brand-new user (with one child) through the real endpoint.
func (s *Stack) Signup() (*Session, string) {
	s.t.Helper()
	n := nextSeq()
	email := fmt.Sprintf("e2e-signup-%d@arunika.test", n)
	const password = "secret123"
	res := s.POST("/auth/signup", "", map[string]interface{}{
		"name":          "E2E Signup",
		"phone_number":  fmt.Sprintf("0813%08d", n),
		"email_address": email,
		"address":       "Jl. E2E",
		"city":          "Jakarta",
		"password":      password,
		"child": []map[string]string{
			{"name": "Budi", "gender": "M", "date_of_birth": "2020-01-02T00:00:00.000"},
		},
	})
	require.Equal(s.t, http.StatusCreated, res.Code, "signup: %s", string(res.Body))
	return sessionFrom(s.t, email, res), password
}

// AdminLogin signs in as the seeded admin and returns the access token.
func (s *Stack) AdminLogin() string {
	s.t.Helper()
	res := s.POST("/admin/auth/login", "", map[string]string{"email": AdminEmail, "password": AdminPassword})
	require.Equal(s.t, http.StatusOK, res.Code, "admin login: %s", string(res.Body))
	tok, ok := res.JSON()["access_token"].(string)
	if !ok {
		tok, ok = res.Data()["access_token"].(string)
	}
	require.True(s.t, ok, "admin login response has no access_token: %s", string(res.Body))
	return tok
}

// PlayPurchase runs the create-order → verify half of a purchase, which is
// everything the backend sees of a Play Billing purchase. path is
// "/payment/play/create-product" or "/payment/play/create"; idKey/id name what
// is being bought; productID is the id the verify call reports.
func (s *Stack) PlayPurchase(sess *Session, createPath, idKey, id, verifyProductID, purchaseToken string) *Resp {
	s.t.Helper()
	created := s.POST(createPath, sess.Token, map[string]string{idKey: id})
	require.Equal(s.t, http.StatusOK, created.Code, "create order: %s", string(created.Body))
	orderID := created.Data()["order_id"].(string)
	return s.POST("/payment/play/verify", sess.Token, map[string]string{
		"order_id":       orderID,
		"product_id":     verifyProductID,
		"purchase_token": purchaseToken,
	})
}

// RTDN posts a Play Real-time Developer Notification the way Pub/Sub push
// delivers it: a base64 payload inside a message envelope.
func (s *Stack) RTDN(notificationType int, subscriptionID, purchaseToken string) *Resp {
	s.t.Helper()
	payload, err := json.Marshal(map[string]interface{}{
		"packageName": "com.arunika",
		"subscriptionNotification": map[string]interface{}{
			"notificationType": notificationType,
			"purchaseToken":    purchaseToken,
			"subscriptionId":   subscriptionID,
		},
	})
	require.NoError(s.t, err)
	return s.POST("/payment/play/rtdn", "", map[string]interface{}{
		"message": map[string]string{"data": base64.StdEncoding.EncodeToString(payload)},
	})
}

// FindByID returns the item with the given id from a list response.
func FindByID(items []map[string]interface{}, id string) map[string]interface{} {
	for _, it := range items {
		if it["id"] == id {
			return it
		}
	}
	return nil
}

// userIDFromToken reads the "sub"/"user_id" style claim out of a JWT payload
// without verifying it — flows only need the id the server already issued.
func userIDFromToken(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3, "not a JWT")
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &claims))
	for _, k := range []string{"user_id", "userID", "sub", "id"} {
		if v, ok := claims[k].(string); ok {
			return v
		}
	}
	t.Fatalf("no user id claim in %v", claims)
	return ""
}
