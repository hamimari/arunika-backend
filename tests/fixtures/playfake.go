package fixtures

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// FakePlay stands in for Google's Android Publisher API.
//
// The backend already supports this: services/google_play_verifier.go reads
// ANDROID_PUBLISHER_BASE_URL specifically so verification can be pointed at a
// test server. That seam is what makes the whole purchase chain — order,
// verification, entitlement grant, content unlock — testable with no Google
// credentials anywhere near the repository.
//
// It also serves the OAuth token endpoint, using a throwaway RSA key
// generated per process. The service account JSON handed to the backend is
// genuinely well-formed and its token_uri points back here, so the real
// oauth2 library performs a real signed-assertion exchange that never leaves
// the machine.
type FakePlay struct {
	Server *httptest.Server
	// ServiceAccountJSON is what GOOGLE_PLAY_SERVICE_ACCOUNT_JSON should be
	// set to for the backend to talk to this server.
	ServiceAccountJSON string

	mu            sync.Mutex
	products      map[string]int    // purchase token -> purchaseState (0 purchased, 1 canceled, 2 pending)
	subscriptions map[string]string // purchase token -> expiryTimeMillis
	voided        []map[string]any
	errors        map[string]int // purchase token -> HTTP status to return instead
}

// NewFakePlay starts the fake server. Callers set the three environment
// variables it reports, normally once per test process, and then register
// per-test purchase tokens — which keeps it usable from parallel tests.
func NewFakePlay() (*FakePlay, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate throwaway key: %w", err)
	}

	f := &FakePlay{
		products:      map[string]int{},
		subscriptions: map[string]string{},
		errors:        map[string]int{},
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handle))

	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	sa := map[string]string{
		"type":           "service_account",
		"project_id":     "arunika-test",
		"private_key_id": "test-key",
		"private_key":    string(pemKey),
		"client_email":   "play-test@arunika-test.iam.gserviceaccount.com",
		"client_id":      "1",
		"token_uri":      f.Server.URL + "/token",
	}
	encoded, err := json.Marshal(sa)
	if err != nil {
		return nil, err
	}
	f.ServiceAccountJSON = string(encoded)
	return f, nil
}

func (f *FakePlay) Close() { f.Server.Close() }

// ─── Registering expectations ─────────────────────────────────────────────────

// Purchased marks a one-time product purchase token as validly purchased.
func (f *FakePlay) Purchased(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.products[token] = 0
}

// Canceled marks a product purchase token as canceled.
func (f *FakePlay) Canceled(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.products[token] = 1
}

// Pending marks a product purchase token as still pending.
func (f *FakePlay) Pending(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.products[token] = 2
}

// SubscriptionActive marks a subscription purchase token as active until
// expiryMillis (a Unix epoch time in milliseconds, as a string).
func (f *FakePlay) SubscriptionActive(token, expiryMillis string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscriptions[token] = expiryMillis
}

// Rejects makes any lookup of this token return the given HTTP status, which
// is how Google reports a token it has never seen.
func (f *FakePlay) Rejects(token string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errors[token] = status
}

// Void adds an entry to the voided-purchases feed used by refund
// reconciliation.
func (f *FakePlay) Void(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.voided = append(f.voided, map[string]any{
		"purchaseToken":    token,
		"voidedTimeMillis": "1700000000000",
	})
}

// ─── Serving ──────────────────────────────────────────────────────────────────

func (f *FakePlay) handle(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/token":
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token": "fake-play-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})

	case strings.Contains(r.URL.Path, "/purchases/voidedpurchases"):
		f.mu.Lock()
		defer f.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"voidedPurchases": f.voided})

	case strings.Contains(r.URL.Path, "/purchases/products/"):
		token := lastSegment(r.URL.Path)
		f.mu.Lock()
		defer f.mu.Unlock()
		if status, bad := f.errors[token]; bad {
			writeJSON(w, status, map[string]any{"error": map[string]any{"message": "not found"}})
			return
		}
		state, known := f.products[token]
		if !known {
			// Google returns 410 Gone for a token it does not recognise.
			writeJSON(w, http.StatusGone, map[string]any{
				"error": map[string]any{"message": "purchase token not found"},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"purchaseState": state,
			"orderId":       "GPA.FAKE-" + token,
		})

	case strings.Contains(r.URL.Path, "/purchases/subscriptions/"):
		token := lastSegment(r.URL.Path)
		f.mu.Lock()
		defer f.mu.Unlock()
		if status, bad := f.errors[token]; bad {
			writeJSON(w, status, map[string]any{"error": map[string]any{"message": "not found"}})
			return
		}
		expiry, known := f.subscriptions[token]
		if !known {
			writeJSON(w, http.StatusGone, map[string]any{
				"error": map[string]any{"message": "purchase token not found"},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"orderId":          "GPA.FAKE-" + token,
			"expiryTimeMillis": expiry,
		})

	case strings.Contains(r.URL.Path, "/externalTransactions"):
		writeJSON(w, http.StatusOK, map[string]any{"externalTransactionId": "fake-ext"})

	default:
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"message": "unhandled path " + r.URL.Path},
		})
	}
}

func lastSegment(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	return parts[len(parts)-1]
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
