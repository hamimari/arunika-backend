package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A syntactically valid service account; the key is never used because the
// token source is pre-populated with a static token in these tests.
const testPlayServiceAccountJSON = `{"type":"service_account","project_id":"arunika-test","private_key_id":"k","private_key":"-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----\n","client_email":"play@arunika-test.iam.gserviceaccount.com","client_id":"1","token_uri":"https://oauth2.googleapis.com/token"}`

type staticTokenSource struct{ token *oauth2.Token }

func (s staticTokenSource) Token() (*oauth2.Token, error) { return s.token, nil }

// newTestVerifier returns a GooglePlayVerifier whose credentials are
// pre-cached with a static token source, so tokenSourceFor's cache-hit path
// is taken and no real Google OAuth/network call is ever made.
func newTestVerifier(t *testing.T, baseURL string) *GooglePlayVerifier {
	t.Setenv("GOOGLE_PLAY_SERVICE_ACCOUNT_JSON", testPlayServiceAccountJSON)
	t.Setenv("ANDROID_PUBLISHER_BASE_URL", baseURL)
	t.Setenv("ANDROID_PACKAGE_NAME", "com.arunika")
	return &GooglePlayVerifier{
		httpClient:  http.DefaultClient,
		credJSON:    testPlayServiceAccountJSON,
		tokenSource: staticTokenSource{token: &oauth2.Token{AccessToken: "test-access-token"}},
	}
}

func TestGooglePlayVerifier_VerifyProductPurchase_Purchased(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/purchases/products/pack_bundle_1/tokens/tok-abc")
		_ = json.NewEncoder(w).Encode(PlayProductPurchase{PurchaseState: PlayPurchaseStatePurchased, OrderID: "GPA.1234"})
	}))
	defer server.Close()

	v := newTestVerifier(t, server.URL)
	result, err := v.VerifyProductPurchase(context.Background(), "pack_bundle_1", "tok-abc")
	require.NoError(t, err)
	assert.Equal(t, PlayPurchaseStatePurchased, result.PurchaseState)
	assert.Equal(t, "GPA.1234", result.OrderID)
}

func TestGooglePlayVerifier_VerifyProductPurchase_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	v := newTestVerifier(t, server.URL)
	_, err := v.VerifyProductPurchase(context.Background(), "pack_bundle_1", "tok-missing")
	assert.Error(t, err)
}

func TestGooglePlayVerifier_VerifySubscriptionPurchase_Active(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/purchases/subscriptions/monthly_premium/tokens/tok-sub")
		_ = json.NewEncoder(w).Encode(PlaySubscriptionPurchase{OrderID: "GPA.5678", ExpiryTimeMillis: "4102444800000"})
	}))
	defer server.Close()

	v := newTestVerifier(t, server.URL)
	result, err := v.VerifySubscriptionPurchase(context.Background(), "monthly_premium", "tok-sub")
	require.NoError(t, err)
	assert.Equal(t, "GPA.5678", result.OrderID)
	assert.Equal(t, "4102444800000", result.ExpiryTimeMillis)
}

func TestGooglePlayVerifier_NotConfigured(t *testing.T) {
	t.Setenv("GOOGLE_PLAY_SERVICE_ACCOUNT_JSON", "")
	v := NewGooglePlayVerifier()
	_, err := v.VerifyProductPurchase(context.Background(), "pack_bundle_1", "tok-abc")
	assert.ErrorIs(t, err, ErrPlayBillingNotConfigured)
}

func TestGooglePlayVerifier_ReportExternalTransaction_OneTime(t *testing.T) {
	var gotBody CreateExternalTransactionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/androidpublisher/v3/applications/com.arunika/externalTransactions")
		assert.Equal(t, "order-123", r.URL.Query().Get("externalTransactionId"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	v := newTestVerifier(t, server.URL)
	req := CreateExternalTransactionRequest{
		OriginalPreTaxAmount: ExternalTransactionPrice{Currency: "IDR", PriceMicros: "29000000000"},
		OriginalTaxAmount:    ExternalTransactionPrice{Currency: "IDR", PriceMicros: "0"},
		TransactionTime:      "2026-09-18T10:00:00Z",
		UserTaxAddress:       ExternalTransactionAddress{RegionCode: "ID"},
		OneTimeTransaction:   &OneTimeExternalTransaction{ExternalTransactionToken: "ext-token-abc"},
	}

	err := v.ReportExternalTransaction(context.Background(), "order-123", req)

	require.NoError(t, err)
	require.NotNil(t, gotBody.OneTimeTransaction)
	assert.Equal(t, "ext-token-abc", gotBody.OneTimeTransaction.ExternalTransactionToken)
	assert.Equal(t, "IDR", gotBody.OriginalPreTaxAmount.Currency)
	assert.Equal(t, "29000000000", gotBody.OriginalPreTaxAmount.PriceMicros)
	assert.Equal(t, "ID", gotBody.UserTaxAddress.RegionCode)
	assert.Nil(t, gotBody.RecurringTransaction)
}

func TestGooglePlayVerifier_ReportExternalTransaction_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid externalTransactionId"}`))
	}))
	defer server.Close()

	v := newTestVerifier(t, server.URL)
	err := v.ReportExternalTransaction(context.Background(), "order-123", CreateExternalTransactionRequest{})
	assert.Error(t, err)
}
