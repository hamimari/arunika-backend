package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ErrPlayBillingNotConfigured is returned when GOOGLE_PLAY_SERVICE_ACCOUNT_JSON
// is unset — there is no persisted purchase to fall back on, so the caller
// must reject the request rather than silently skip verification.
var ErrPlayBillingNotConfigured = fmt.Errorf("Google Play Billing is not configured (GOOGLE_PLAY_SERVICE_ACCOUNT_JSON is empty)")

// androidPublisherBaseURL is a var so tests can point verification at an
// httptest server instead of the real Android Publisher API.
var androidPublisherBaseURL = func() string {
	if url := os.Getenv("ANDROID_PUBLISHER_BASE_URL"); url != "" {
		return url
	}
	return "https://androidpublisher.googleapis.com"
}

// androidPackageName returns the Android application ID used in Play
// Developer API paths — defaults to the app's real applicationId.
func androidPackageName() string {
	if v := os.Getenv("ANDROID_PACKAGE_NAME"); v != "" {
		return v
	}
	return "com.arunika"
}

// PlayPurchaseState mirrors Android Publisher's purchases.products
// purchaseState field: 0 = purchased, 1 = canceled, 2 = pending.
type PlayPurchaseState int

const (
	PlayPurchaseStatePurchased PlayPurchaseState = 0
	PlayPurchaseStateCanceled  PlayPurchaseState = 1
	PlayPurchaseStatePending   PlayPurchaseState = 2
)

// PlayProductPurchase mirrors the fields needed from
// purchases.products.get for a one-time ("content") package.
type PlayProductPurchase struct {
	PurchaseState PlayPurchaseState `json:"purchaseState"`
	OrderID       string            `json:"orderId"`
}

// PlaySubscriptionPurchase mirrors the fields needed from
// purchases.subscriptions.get for a "subscription" package.
type PlaySubscriptionPurchase struct {
	OrderID          string `json:"orderId"`
	ExpiryTimeMillis string `json:"expiryTimeMillis"`
	// CancelReason is present once the subscription has been canceled; its
	// absence does not mean active — ExpiryTimeMillis is the source of truth.
	CancelReason *int `json:"cancelReason"`
}

// ExternalTransactionPrice mirrors the Android Publisher API's Price type —
// used to report the pre-tax amount and tax amount of a transaction settled
// outside Google Play Billing (see ReportExternalTransaction).
type ExternalTransactionPrice struct {
	Currency    string `json:"currency"`
	PriceMicros string `json:"priceMicros"`
}

// ExternalTransactionAddress mirrors ExternalTransactionAddress — only
// regionCode is needed for Indonesia (administrativeArea is India-only).
type ExternalTransactionAddress struct {
	RegionCode string `json:"regionCode"`
}

// OneTimeExternalTransaction mirrors OneTimeExternalTransaction, used when
// reporting a one-time ("content") package purchase.
type OneTimeExternalTransaction struct {
	ExternalTransactionToken string `json:"externalTransactionToken"`
}

// ExternalSubscription mirrors ExternalSubscription.subscriptionType.
type ExternalSubscription struct {
	// One of SUBSCRIPTION_TYPE_UNSPECIFIED, RECURRING, PREPAID.
	SubscriptionType string `json:"subscriptionType"`
}

// RecurringExternalTransaction mirrors RecurringExternalTransaction, used
// when reporting a "subscription" package purchase.
type RecurringExternalTransaction struct {
	ExternalTransactionToken string               `json:"externalTransactionToken"`
	ExternalSubscription     ExternalSubscription `json:"externalSubscription"`
}

// CreateExternalTransactionRequest mirrors the ExternalTransaction resource
// fields required to create one — see
// https://developers.google.com/android-publisher/api-ref/rest/v3/externaltransactions/createexternaltransaction.
type CreateExternalTransactionRequest struct {
	OriginalPreTaxAmount ExternalTransactionPrice      `json:"originalPreTaxAmount"`
	OriginalTaxAmount    ExternalTransactionPrice      `json:"originalTaxAmount"`
	TransactionTime      string                        `json:"transactionTime"`
	UserTaxAddress       ExternalTransactionAddress    `json:"userTaxAddress"`
	OneTimeTransaction   *OneTimeExternalTransaction   `json:"oneTimeTransaction,omitempty"`
	RecurringTransaction *RecurringExternalTransaction `json:"recurringTransaction,omitempty"`
}

// GooglePlayVerifier calls the Android Publisher API to verify purchase
// tokens reported by the app after a Google Play Billing purchase.
// playPurchaseVerifier is the subset of GooglePlayVerifier that
// PaymentService depends on — narrowed to an interface so tests can inject
// a fake instead of exercising real Google OAuth/HTTP calls.
type playPurchaseVerifier interface {
	VerifyProductPurchase(ctx context.Context, productID, purchaseToken string) (*PlayProductPurchase, error)
	VerifySubscriptionPurchase(ctx context.Context, subscriptionID, purchaseToken string) (*PlaySubscriptionPurchase, error)
	ReportExternalTransaction(ctx context.Context, externalTransactionID string, req CreateExternalTransactionRequest) error
	ListVoidedPurchases(ctx context.Context, since time.Time) ([]VoidedPurchase, error)
}

// VoidedPurchase mirrors the fields needed from
// purchases.voidedpurchases.list — see
// https://developers.google.com/android-publisher/api-ref/rest/v3/purchases.voidedpurchases/list.
// purchaseSpecifiedType: 0 = subscription, 1 = one-time product.
type VoidedPurchase struct {
	PurchaseToken         string `json:"purchaseToken"`
	OrderID               string `json:"orderId"`
	VoidedTimeMillis      string `json:"voidedTimeMillis"`
	VoidedSource          int    `json:"voidedSource"`
	VoidedReason          int    `json:"voidedReason"`
	PurchaseSpecifiedType int    `json:"purchaseSpecifiedType"`
}

type voidedPurchasesResponse struct {
	VoidedPurchases []VoidedPurchase `json:"voidedPurchases"`
	TokenPagination struct {
		NextPageToken string `json:"nextPageToken"`
	} `json:"tokenPagination"`
}

type GooglePlayVerifier struct {
	httpClient *http.Client

	// Credentials are parsed once per distinct service-account JSON and
	// reused; the Google token source caches the OAuth token until expiry.
	credMu      sync.Mutex
	credJSON    string
	tokenSource oauth2.TokenSource
}

func NewGooglePlayVerifier() *GooglePlayVerifier {
	return &GooglePlayVerifier{httpClient: http.DefaultClient}
}

func (v *GooglePlayVerifier) tokenSourceFor(ctx context.Context) (oauth2.TokenSource, bool, error) {
	saJSON, err := loadServiceAccountJSONFromEnv("GOOGLE_PLAY_SERVICE_ACCOUNT_JSON")
	if err != nil {
		return nil, false, err
	}
	if saJSON == "" {
		return nil, false, nil
	}

	v.credMu.Lock()
	defer v.credMu.Unlock()
	if v.tokenSource != nil && v.credJSON == saJSON {
		return v.tokenSource, true, nil
	}

	creds, err := google.CredentialsFromJSON(ctx, []byte(saJSON), "https://www.googleapis.com/auth/androidpublisher")
	if err != nil {
		return nil, false, fmt.Errorf("parse service account: %w", err)
	}
	v.credJSON, v.tokenSource = saJSON, creds.TokenSource
	return v.tokenSource, true, nil
}

func (v *GooglePlayVerifier) get(ctx context.Context, path string, out interface{}) error {
	tokenSource, ok, err := v.tokenSourceFor(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPlayBillingNotConfigured
	}
	token, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("get oauth token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, androidPublisherBaseURL()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("android publisher request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("android publisher error %d: %s", resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (v *GooglePlayVerifier) post(ctx context.Context, path string, body interface{}) error {
	tokenSource, ok, err := v.tokenSourceFor(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPlayBillingNotConfigured
	}
	token, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("get oauth token: %w", err)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, androidPublisherBaseURL()+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("android publisher request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("android publisher error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ReportExternalTransaction reports a purchase settled outside Google Play
// Billing (via Midtrans, chosen by the user in the User Choice Billing
// selection screen) to Google — required within 24 hours of the
// transaction so Play can calculate its service fee.
// externalTransactionID must be unique per transaction; callers pass their
// internal order ID.
func (v *GooglePlayVerifier) ReportExternalTransaction(ctx context.Context, externalTransactionID string, req CreateExternalTransactionRequest) error {
	path := fmt.Sprintf("/androidpublisher/v3/applications/%s/externalTransactions?externalTransactionId=%s",
		androidPackageName(), url.QueryEscape(externalTransactionID))
	return v.post(ctx, path, req)
}

// VerifyProductPurchase verifies a one-time ("content") package purchase.
func (v *GooglePlayVerifier) VerifyProductPurchase(ctx context.Context, productID, purchaseToken string) (*PlayProductPurchase, error) {
	path := fmt.Sprintf("/androidpublisher/v3/applications/%s/purchases/products/%s/tokens/%s",
		androidPackageName(), productID, purchaseToken)
	var out PlayProductPurchase
	if err := v.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VerifySubscriptionPurchase verifies an auto-renewing subscription purchase.
func (v *GooglePlayVerifier) VerifySubscriptionPurchase(ctx context.Context, subscriptionID, purchaseToken string) (*PlaySubscriptionPurchase, error) {
	path := fmt.Sprintf("/androidpublisher/v3/applications/%s/purchases/subscriptions/%s/tokens/%s",
		androidPackageName(), subscriptionID, purchaseToken)
	var out PlaySubscriptionPurchase
	if err := v.get(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListVoidedPurchases returns purchases voided (refunded, canceled, or
// charged back) since [since] — including Google's own automatic refund of
// any purchase left unacknowledged for 3 days. This is Google's documented
// backend safety net for catching entitlement-affecting events that RTDN
// isn't guaranteed to cover (e.g. one-time product refunds), so it must be
// polled periodically rather than relied on as a push notification — see
// PaymentService.ReconcileVoidedPurchases. Play's own window for this
// endpoint tops out well under a year, so this is meant to run at least
// every few days, not once.
//
// NOTE: the `type` query parameter's exact semantics should be verified
// against Google's live API reference before relying on this in
// production — implemented here from the schema as last fetched (see the
// same caveat already on ReportExternalTransaction).
func (v *GooglePlayVerifier) ListVoidedPurchases(ctx context.Context, since time.Time) ([]VoidedPurchase, error) {
	var all []VoidedPurchase
	pageToken := ""
	for {
		path := fmt.Sprintf("/androidpublisher/v3/applications/%s/purchases/voidedpurchases?startTime=%d&maxResults=1000&type=1",
			androidPackageName(), since.UnixMilli())
		if pageToken != "" {
			path += "&token=" + url.QueryEscape(pageToken)
		}
		var out voidedPurchasesResponse
		if err := v.get(ctx, path, &out); err != nil {
			return nil, err
		}
		all = append(all, out.VoidedPurchases...)
		if out.TokenPagination.NextPageToken == "" {
			break
		}
		pageToken = out.TokenPagination.NextPageToken
	}
	return all, nil
}
