package fixtures

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
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
	// AdvertisedURL is what ANDROID_PUBLISHER_BASE_URL should be set to.
	// Equal to Server.URL for NewFakePlay (same-process callers); for
	// NewFakePlayForDocker it names the host a containerized backend can
	// actually dial, not the 0.0.0.0 the listener is bound to.
	AdvertisedURL string

	mu            sync.Mutex
	products      map[string]int    // purchase token -> purchaseState (0 purchased, 1 canceled, 2 pending)
	subscriptions map[string]string // purchase token -> expiryTimeMillis
	voided        []map[string]any
	errors        map[string]int // purchase token -> HTTP status to return instead

	refundedOrders map[string]bool   // Play order id -> refunded via orders.refund
	revokedTokens  map[string]string // subscription token -> "fullRefund" | "proratedRefund"
	refuse         map[string]int    // Play order id or token -> HTTP status for a refund/revoke
	orderTotals    map[string]string // Play order id -> total units (IDR) orders.get reports

	// acceptPrefix, when set, makes any unregistered product token with this
	// prefix a valid purchase — for suites outside this process (Playwright)
	// that can't register tokens themselves.
	acceptPrefix string
}

// NewFakePlay starts the fake server bound to loopback only, for tests that
// run in the same process as the backend code under test (tests/api,
// tests/db). Callers set the three environment variables it reports, then
// register per-test purchase tokens — which keeps it usable from parallel
// tests.
func NewFakePlay() (*FakePlay, error) {
	return newFakePlay("127.0.0.1:0", "")
}

// NewFakePlayForDocker starts the fake server bound to every interface
// (0.0.0.0), for the cross-system E2E suite: there the backend runs inside a
// container and reaches this server over the host network, so loopback would
// resolve to the container itself, not the host. advertiseHost is the name
// the containerized backend can reach this process by — normally
// "host.docker.internal" (works out of the box on Docker Desktop and OrbStack;
// docker-compose.test.yml adds the extra_hosts entry that makes it resolve on
// Linux CI runners too).
func NewFakePlayForDocker(advertiseHost string) (*FakePlay, error) {
	return newFakePlay("0.0.0.0:0", advertiseHost)
}

func newFakePlay(listenAddr, advertiseHost string) (*FakePlay, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate throwaway key: %w", err)
	}

	f := &FakePlay{
		products:       map[string]int{},
		subscriptions:  map[string]string{},
		errors:         map[string]int{},
		refundedOrders: map[string]bool{},
		revokedTokens:  map[string]string{},
		refuse:         map[string]int{},
		orderTotals:    map[string]string{},
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", listenAddr, err)
	}
	f.Server = &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: http.HandlerFunc(f.handle)},
	}
	f.Server.Start()

	// The URL Start() derives is built from the listener's own address —
	// 0.0.0.0 for the Docker case, which nothing outside this machine can
	// dial. Rewrite it to the address a container can actually reach.
	advertisedURL := f.Server.URL
	if advertiseHost != "" {
		_, port, splitErr := net.SplitHostPort(listener.Addr().String())
		if splitErr != nil {
			return nil, fmt.Errorf("split listener address: %w", splitErr)
		}
		advertisedURL = "http://" + net.JoinHostPort(advertiseHost, port)
	}

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
		"token_uri":      advertisedURL + "/token",
	}
	encoded, err := json.Marshal(sa)
	if err != nil {
		return nil, err
	}
	f.ServiceAccountJSON = string(encoded)
	f.AdvertisedURL = advertisedURL
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

// AcceptPurchasesWithPrefix treats any product purchase token starting with
// prefix as validly purchased, without registering it first.
func (f *FakePlay) AcceptPurchasesWithPrefix(prefix string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acceptPrefix = prefix
}

// PlayOrderID is the Play order id this fake reports for a purchase token.
func PlayOrderID(token string) string { return "GPA.FAKE-" + token }

// OrderTotal sets the total (in IDR units) orders.get reports for an order.
func (f *FakePlay) OrderTotal(playOrderID, units string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orderTotals[playOrderID] = units
}

// RefuseRefund makes a refund (by Play order id) or revoke (by purchase
// token) fail with the given HTTP status.
func (f *FakePlay) RefuseRefund(idOrToken string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuse[idOrToken] = status
}

// AlreadyRefunded models an order refunded outside the app (e.g. in Play
// Console): Google refuses a second refund, and orders.get reports REFUNDED.
func (f *FakePlay) AlreadyRefunded(playOrderID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refundedOrders[playOrderID] = true
	f.refuse[playOrderID] = http.StatusBadRequest
}

// Refunded reports whether orders.refund was called for the Play order id.
func (f *FakePlay) Refunded(playOrderID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refundedOrders[playOrderID]
}

// Revoked returns how a subscription token was revoked ("" if it wasn't).
func (f *FakePlay) Revoked(token string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.revokedTokens[token]
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

	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":refund") && strings.Contains(r.URL.Path, "/orders/"):
		id := strings.TrimSuffix(lastSegment(r.URL.Path), ":refund")
		f.mu.Lock()
		defer f.mu.Unlock()
		if status, refused := f.refuse[id]; refused {
			writeJSON(w, status, map[string]any{"error": map[string]any{"message": "refund not allowed"}})
			return
		}
		f.refundedOrders[id] = true
		writeJSON(w, http.StatusOK, map[string]any{})

	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":revoke") && strings.Contains(r.URL.Path, "/purchases/subscriptionsv2/tokens/"):
		token := strings.TrimSuffix(lastSegment(r.URL.Path), ":revoke")
		var body struct {
			RevocationContext map[string]any `json:"revocationContext"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if status, refused := f.refuse[token]; refused {
			writeJSON(w, status, map[string]any{"error": map[string]any{"message": "revoke not allowed"}})
			return
		}
		kind := "fullRefund"
		if _, prorated := body.RevocationContext["proratedRefund"]; prorated {
			kind = "proratedRefund"
		}
		f.revokedTokens[token] = kind
		writeJSON(w, http.StatusOK, map[string]any{})

	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/orders/"):
		id := lastSegment(r.URL.Path)
		f.mu.Lock()
		defer f.mu.Unlock()
		total := f.orderTotals[id]
		if total == "" {
			total = "0"
		}
		refunded := f.refundedOrders[id] || f.revokedTokens[strings.TrimPrefix(id, "GPA.FAKE-")] != ""
		order := map[string]any{
			"orderId": id,
			"state":   "PROCESSED",
			"total":   map[string]any{"currencyCode": "IDR", "units": total},
			"tax":     map[string]any{"currencyCode": "IDR", "units": "0"},
		}
		if refunded {
			order["state"] = "REFUNDED"
			order["orderHistory"] = map[string]any{"refundEvent": map[string]any{
				"eventTime":    "2026-09-28T00:00:00Z",
				"refundReason": "OTHER",
				"refundDetails": map[string]any{
					"total": map[string]any{"currencyCode": "IDR", "units": total},
					"tax":   map[string]any{"currencyCode": "IDR", "units": "0"},
				},
			}}
		}
		writeJSON(w, http.StatusOK, order)

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
		if !known && f.acceptPrefix != "" && strings.HasPrefix(token, f.acceptPrefix) {
			state, known = 0, true
		}
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
