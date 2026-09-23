package services

import (
	"arunika_backend/models"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentService struct {
	db                 *gorm.DB
	entitlementService *EntitlementService
	playVerifier       playPurchaseVerifier
}

func NewPaymentService(db *gorm.DB, entitlementService *EntitlementService) *PaymentService {
	return &PaymentService{db: db, entitlementService: entitlementService, playVerifier: NewGooglePlayVerifier()}
}

// midtransBaseURL returns the Midtrans Snap API base URL, env-driven so
// sandbox/prod can differ without a code change (defaults to sandbox).
func midtransBaseURL() string {
	if url := os.Getenv("MIDTRANS_BASE_URL"); url != "" {
		return url
	}
	return "https://app.sandbox.midtrans.com"
}

// midtransCoreAPIBaseURL returns the Midtrans Core API base URL, used for
// actively querying a transaction's status (distinct from the Snap API used
// to create transactions).
func midtransCoreAPIBaseURL() string {
	if url := os.Getenv("MIDTRANS_CORE_API_URL"); url != "" {
		return url
	}
	return "https://api.sandbox.midtrans.com"
}

type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
	OrderID     string `json:"order_id"`
}

// CreateSnapTransaction creates an Order (status PENDING) and its initial
// Payment audit row (status "pending"), then a Midtrans Snap transaction for
// it. The Midtrans-facing order_id is derived from the Order's own ID
// (order-<uuid>) — the webhook uses this to look the Order back up and drive
// settlement, instead of the client's own callback. Recording the Payment row
// up front (not just on webhook receipt) means a payment attempt that never
// gets a webhook (abandoned checkout, dropped notification) still leaves an
// audit trail instead of vanishing.
func (s *PaymentService) CreateSnapTransaction(user *models.Parent, premiumPackage *models.PremiumPackage) (*SnapResponse, error) {
	packageID := uuid.MustParse(premiumPackage.ID)
	return s.createOrderAndSnapTransaction(user, int64(premiumPackage.PriceIdr), premiumPackage.ID, premiumPackage.Name, &packageID, nil)
}

// CreateSnapTransactionForProduct is CreateSnapTransaction's counterpart for
// a single-product purchase (order.product_id set instead of order.package_id).
func (s *PaymentService) CreateSnapTransactionForProduct(user *models.Parent, product *models.Product, itemName string) (*SnapResponse, error) {
	return s.createOrderAndSnapTransaction(user, product.PriceIdr, product.ID.String(), itemName, nil, &product.ID)
}

func (s *PaymentService) createOrderAndSnapTransaction(user *models.Parent, amountIdr int64, itemID, itemName string, packageID, productID *uuid.UUID) (*SnapResponse, error) {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		return nil, fmt.Errorf("MIDTRANS_SERVER_KEY not configured")
	}

	var order models.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order = models.Order{
			UserID:    user.ID,
			PackageID: packageID,
			ProductID: productID,
			AmountIdr: amountIdr,
			Status:    models.OrderStatusPending,
			Provider:  models.OrderProviderMidtrans,
		}
		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("create order: %w", err)
		}

		payment := models.Payment{
			OrderID:           order.ID,
			ProviderOrderID:   fmt.Sprintf("order-%s", order.ID.String()),
			UserID:            &user.ID,
			TransactionStatus: "pending",
			GrossAmount:       fmt.Sprintf("%d.00", amountIdr),
		}
		if err := tx.Create(&payment).Error; err != nil {
			return fmt.Errorf("create payment: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	providerOrderID := fmt.Sprintf("order-%s", order.ID.String())

	payload := map[string]interface{}{
		"transaction_details": map[string]interface{}{
			"order_id":     providerOrderID,
			"gross_amount": amountIdr,
		},
		"customer_details": map[string]interface{}{
			"first_name": user.Name,
			"email":      user.EmailAddress,
			"phone":      user.PhoneNumber,
		},
		"item_details": []map[string]interface{}{
			{
				"id":       itemID,
				"price":    amountIdr,
				"quantity": 1,
				"name":     itemName,
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", midtransBaseURL()+"/snap/v1/transactions", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(serverKey, "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("midtrans request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("midtrans error %d: %s", resp.StatusCode, string(b))
	}

	var snapResp SnapResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapResp); err != nil {
		return nil, fmt.Errorf("decode snap response: %w", err)
	}
	snapResp.OrderID = order.ID.String()

	if err := s.db.Model(&models.Payment{}).Where("order_id = ?", order.ID).
		Update("payment_url", snapResp.RedirectURL).Error; err != nil {
		return nil, fmt.Errorf("save payment url: %w", err)
	}

	return &snapResp, nil
}

// ValidateWebhookSignature validates the Midtrans webhook notification signature.
// Signature: SHA-512(order_id + status_code + gross_amount + server_key)
func ValidateWebhookSignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	raw := orderID + statusCode + grossAmount + serverKey
	h := sha512.New()
	h.Write([]byte(raw))
	expected := hex.EncodeToString(h.Sum(nil))
	return expected == signatureKey
}

type WebhookNotification struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	TransactionID     string `json:"transaction_id"`
	PaymentType       string `json:"payment_type"`
}

// parseProviderOrderID extracts our internal order UUID from the
// "order-<uuid>" identifier we generated in CreateSnapTransaction.
func parseProviderOrderID(providerOrderID string) (uuid.UUID, error) {
	id, ok := strings.CutPrefix(providerOrderID, "order-")
	if !ok {
		return uuid.Nil, fmt.Errorf("unrecognized order id format: %s", providerOrderID)
	}
	return uuid.Parse(id)
}

// HandleWebhook processes a Midtrans payment notification: it validates the
// signature, resolves the order id, and delegates to applyTransactionStatus
// to record the callback and (idempotently) settle the order.
// Returns userID so the caller can dispatch a push notification.
func (s *PaymentService) HandleWebhook(notif WebhookNotification) (uuid.UUID, error) {
	if !ValidateWebhookSignature(notif.OrderID, notif.StatusCode, notif.GrossAmount, notif.SignatureKey) {
		return uuid.Nil, fmt.Errorf("invalid signature")
	}

	orderID, err := parseProviderOrderID(notif.OrderID)
	if err != nil {
		return uuid.Nil, err
	}

	rawPayload, _ := json.Marshal(notif)
	order, err := s.applyTransactionStatus(orderID, notif.OrderID, notif.TransactionStatus, notif.FraudStatus,
		notif.StatusCode, notif.GrossAmount, notif.TransactionID, notif.PaymentType, string(rawPayload))
	if err != nil {
		return uuid.Nil, err
	}
	return order.UserID, nil
}

// applyTransactionStatus locks the order, always records the transaction
// state as a new Payment audit row, and — only if the order is still
// PENDING, inside the same DB transaction — transitions it and grants
// entitlements. Replaying the same status after the order is no longer
// PENDING is a no-op (idempotent). Shared by HandleWebhook and
// SyncOrderStatus so a webhook delivery and an active Midtrans status check
// always converge on the same outcome.
func (s *PaymentService) applyTransactionStatus(orderID uuid.UUID, providerOrderID, transactionStatus, fraudStatus, statusCode, grossAmount, transactionID, paymentType, rawPayload string) (*models.Order, error) {
	var result models.Order

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", orderID).First(&order).Error; err != nil {
			return fmt.Errorf("order not found for %s: %w", providerOrderID, err)
		}
		result = order

		payment := models.Payment{
			OrderID:           order.ID,
			ProviderOrderID:   providerOrderID,
			UserID:            &order.UserID,
			TransactionID:     transactionID,
			TransactionStatus: transactionStatus,
			PaymentType:       paymentType,
			GrossAmount:       grossAmount,
			StatusCode:        statusCode,
			FraudStatus:       fraudStatus,
			RawPayload:        rawPayload,
		}
		if err := tx.Create(&payment).Error; err != nil {
			return fmt.Errorf("save payment: %w", err)
		}

		// Idempotency guard: only the first settlement/failure transitions
		// the order and grants access. Replays after that are no-ops.
		if order.Status != models.OrderStatusPending {
			return nil
		}

		settled := (transactionStatus == "settlement") ||
			(transactionStatus == "capture" && fraudStatus == "accept")

		var newStatus string
		switch {
		case settled:
			newStatus = models.OrderStatusPaid
		case transactionStatus == "expire":
			newStatus = models.OrderStatusExpired
		case transactionStatus == "deny" || transactionStatus == "cancel":
			newStatus = models.OrderStatusFailed
		default:
			return nil // e.g. Midtrans's own "pending" — nothing to do yet
		}

		if err := tx.Model(&order).Update("status", newStatus).Error; err != nil {
			return fmt.Errorf("update order status: %w", err)
		}
		order.Status = newStatus
		result = order

		if newStatus == models.OrderStatusPaid {
			if err := s.entitlementService.GrantForPaidOrder(tx, &order); err != nil {
				return fmt.Errorf("grant entitlements: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type midtransStatusResponse struct {
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	TransactionID     string `json:"transaction_id"`
	PaymentType       string `json:"payment_type"`
}

// SyncOrderStatus actively checks Midtrans's Core API for a still-PENDING
// order's true transaction status, instead of relying solely on the webhook.
// This closes the gap where a delayed or dropped webhook notification would
// otherwise leave the client's poll of GET /orders/:id timing out against a
// stale PENDING row: any settlement/failure discovered here is applied via
// the same applyTransactionStatus path a webhook delivery would use. It is
// best-effort and fails open — any error talking to Midtrans just returns
// the order's last known DB state, so a transient Midtrans/network issue
// never turns a read into a 500.
func (s *PaymentService) SyncOrderStatus(order *models.Order) *models.Order {
	if order.Status != models.OrderStatusPending {
		return order
	}

	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		return order
	}

	providerOrderID := fmt.Sprintf("order-%s", order.ID.String())
	req, err := http.NewRequest("GET", midtransCoreAPIBaseURL()+"/v2/"+providerOrderID+"/status", nil)
	if err != nil {
		return order
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(serverKey, "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("SyncOrderStatus: midtrans status request failed", "order_id", order.ID, "error", err)
		return order
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return order
	}

	var statusResp midtransStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return order
	}

	// Nothing new to reconcile — Midtrans still reports pending (or the
	// order hasn't been opened in the Snap page yet).
	if statusResp.TransactionStatus == "" || statusResp.TransactionStatus == "pending" {
		return order
	}

	rawPayload, _ := json.Marshal(statusResp)
	updated, err := s.applyTransactionStatus(order.ID, providerOrderID, statusResp.TransactionStatus, statusResp.FraudStatus,
		statusResp.StatusCode, statusResp.GrossAmount, statusResp.TransactionID, statusResp.PaymentType, string(rawPayload))
	if err != nil {
		slog.Error("SyncOrderStatus: failed to apply transaction status", "order_id", order.ID, "error", err)
		return order
	}
	return updated
}

// GetSubscription returns the subscription record for a user.
func (s *PaymentService) GetSubscription(userID uuid.UUID) (*models.UserSubscription, error) {
	var sub models.UserSubscription
	if err := s.db.Where("user_id = ?", userID).First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

// ── Google Play Billing ─────────────────────────────────────────────────────
//
// Purchases initiated from the Android app follow the same two-step shape as
// the Midtrans flow above: an Order is created up front (PENDING), the app
// then completes the purchase with Google Play directly, and finally reports
// the resulting purchase token back here to be verified and settled — mirror
// of createOrderAndSnapTransaction + applyTransactionStatus, but verified
// against the Android Publisher API instead of a webhook signature.

// ErrPackageNotPlayMapped is returned when a package has no play_product_id set.
var ErrPackageNotPlayMapped = fmt.Errorf("package is not mapped to a Google Play product")

// ErrProductNotPlayMapped is returned when a product has no play_product_id set.
var ErrProductNotPlayMapped = fmt.Errorf("product is not mapped to a Google Play product")

// CreatePlayOrder creates a PENDING order (and its audit Payment row) for a
// package the app is about to purchase via Google Play Billing. The
// package must already have a play_product_id mapping.
func (s *PaymentService) CreatePlayOrder(user *models.Parent, pkg *models.PremiumPackage) (*models.Order, error) {
	if pkg.PlayProductID == nil || *pkg.PlayProductID == "" {
		return nil, ErrPackageNotPlayMapped
	}

	packageID := uuid.MustParse(pkg.ID)
	var order models.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order = models.Order{
			UserID:    user.ID,
			PackageID: &packageID,
			AmountIdr: int64(pkg.PriceIdr),
			Status:    models.OrderStatusPending,
			Provider:  models.OrderProviderGooglePlay,
		}
		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("create order: %w", err)
		}
		payment := models.Payment{
			OrderID:           order.ID,
			ProviderOrderID:   fmt.Sprintf("order-%s", order.ID.String()),
			UserID:            &user.ID,
			TransactionStatus: "pending",
			PaymentType:       "google_play",
			GrossAmount:       fmt.Sprintf("%d.00", pkg.PriceIdr),
		}
		if err := tx.Create(&payment).Error; err != nil {
			return fmt.Errorf("create payment: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// CreatePlayProductOrder creates a PENDING order (and its audit Payment row) for a
// product the app is about to purchase via Google Play Billing. The
// product must already have a play_product_id mapping.
func (s *PaymentService) CreatePlayProductOrder(user *models.Parent, product *models.Product) (*models.Order, error) {
	if product.PlayProductID == nil || *product.PlayProductID == "" {
		return nil, ErrProductNotPlayMapped
	}

	productID := product.ID
	var order models.Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order = models.Order{
			UserID:    user.ID,
			ProductID: &productID,
			AmountIdr: product.PriceIdr,
			Status:    models.OrderStatusPending,
			Provider:  models.OrderProviderGooglePlay,
		}
		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("create order: %w", err)
		}
		payment := models.Payment{
			OrderID:           order.ID,
			ProviderOrderID:   fmt.Sprintf("order-%s", order.ID.String()),
			UserID:            &user.ID,
			TransactionStatus: "pending",
			PaymentType:       "google_play",
			GrossAmount:       fmt.Sprintf("%d.00", product.PriceIdr),
		}
		if err := tx.Create(&payment).Error; err != nil {
			return fmt.Errorf("create payment: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// ResolvePendingPlayOrder finds the most recent PENDING order the user has
// against the package mapped to playProductID. It recovers an order id the
// app lost track of client-side — e.g. Google Play finished the purchase
// but the app was killed (or lost network) before it could report the
// order id back alongside the purchase token. Per Google's guidance to
// reconcile purchases on every app start rather than only within the flow
// that began them, the app re-submits {product_id, purchase_token} without
// an order id in that case and relies on this lookup instead.
func (s *PaymentService) ResolvePendingPlayOrder(userID uuid.UUID, playProductID string) (uuid.UUID, error) {
	pkg, err := models.FindPremiumPackageByPlayProductID(s.db, playProductID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("find package: %w", err)
	}
	packageID, err := uuid.Parse(pkg.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse package id: %w", err)
	}

	var order models.Order
	err = s.db.Where("user_id = ? AND package_id = ? AND status = ?", userID, packageID, models.OrderStatusPending).
		Order("created_at DESC").
		First(&order).Error
	if err != nil {
		return uuid.Nil, fmt.Errorf("no pending order for this product: %w", err)
	}
	return order.ID, nil
}

// ReconcileVoidedPurchases polls Google Play's Voided Purchases API for
// purchases refunded, canceled, or charged back since [since] — including
// Google's own automatic refund of any purchase left unacknowledged for 3
// days — and revokes entitlement for any of our PAID orders among them.
// This is the backend-side safety net for events RTDN might miss entirely
// (it only covers subscription lifecycle notifications reliably) or that
// happen precisely because the app-side acknowledgment never landed, so it
// must be called periodically (see main.go's startPlayPurchaseReconciliation)
// rather than relied on as a push. Returns the number of orders revoked.
func (s *PaymentService) ReconcileVoidedPurchases(ctx context.Context, since time.Time) (int, error) {
	voided, err := s.playVerifier.ListVoidedPurchases(ctx, since)
	if err != nil {
		return 0, fmt.Errorf("list voided purchases: %w", err)
	}

	reconciled := 0
	for _, v := range voided {
		did, err := s.reconcileVoidedPurchase(v)
		if err != nil {
			slog.Error("ReconcileVoidedPurchases: failed to reconcile", "purchase_token", v.PurchaseToken, "error", err)
			continue
		}
		if did {
			reconciled++
		}
	}
	return reconciled, nil
}

// reconcileVoidedPurchase revokes entitlement for the order behind a single
// voided purchase token, if we have one on file and it's still PAID.
// Returns false (no error) when the token is unknown to us or was already
// reconciled — both are expected steady-state outcomes, not failures.
func (s *PaymentService) reconcileVoidedPurchase(v VoidedPurchase) (bool, error) {
	var payment models.Payment
	err := s.db.Where("transaction_id = ? AND payment_type = ?", v.PurchaseToken, "google_play").First(&payment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	order, err := models.FindOrderByID(s.db, payment.OrderID)
	if err != nil {
		return false, err
	}
	if order.Status != models.OrderStatusPaid {
		return false, nil
	}

	if err := s.db.Model(order).Update("status", models.OrderStatusRefunded).Error; err != nil {
		return false, fmt.Errorf("update order status: %w", err)
	}

	if order.PackageID != nil {
		pkg, err := models.FindPremiumPackageByID(s.db, order.PackageID.String())
		if err != nil {
			return false, fmt.Errorf("load package: %w", err)
		}
		if pkg.Type == "subscription" {
			if err := s.entitlementService.RevokeSubscription(order.UserID); err != nil {
				return false, fmt.Errorf("revoke subscription: %w", err)
			}
			return true, nil
		}
	}
	if err := s.entitlementService.RevokeEntitlementForOrder(order.ID); err != nil {
		return false, fmt.Errorf("revoke entitlement: %w", err)
	}
	return true, nil
}

// ErrPlayPurchaseTokenReused is returned when a purchase token has already
// been applied to a different order than the one being verified.
var ErrPlayPurchaseTokenReused = fmt.Errorf("purchase token already used for another order")

// ErrPlayPurchaseNotValid is returned when Google Play reports the purchase
// as not (or no longer) purchased/active.
var ErrPlayPurchaseNotValid = fmt.Errorf("purchase is not valid")

// VerifyPlayPurchase verifies a Google Play Billing purchase for orderID
// (created via CreatePlayOrder) and, if valid, settles the order and grants
// entitlement — the Google Play analogue of applyTransactionStatus. It is
// idempotent: replaying the same purchase token against the same order that
// is already PAID simply returns the order unchanged.
func (s *PaymentService) VerifyPlayPurchase(ctx context.Context, orderID, userID uuid.UUID, purchaseToken string) (*models.Order, error) {
	// Record the token immediately, as its own statement outside the
	// transaction below — so it survives even if verification against
	// Google fails or a later step (entitlement grant) errors and rolls
	// everything else back. Without this, a support case with a genuinely
	// stuck order would need the user to reproduce the purchase just to
	// hand support a token again; with it, an admin's later re-sync can
	// just reuse what's already on file. Best-effort: a failure here
	// doesn't block verification itself.
	if err := s.db.Model(&models.Order{}).
		Where("id = ? AND user_id = ?", orderID, userID).
		Update("purchase_token", purchaseToken).Error; err != nil {
		slog.Warn("VerifyPlayPurchase: failed to persist purchase token", "order_id", orderID, "error", err)
	}

	var result models.Order

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
			return fmt.Errorf("order not found: %w", err)
		}
		result = order

		if order.Status == models.OrderStatusPaid {
			// Idempotent replay: only accept if it's the same token that
			// already settled this order.
			var existing models.Payment
			if err := tx.Where("order_id = ? AND transaction_id = ?", order.ID, purchaseToken).First(&existing).Error; err == nil {
				return nil
			}
			return fmt.Errorf("order already settled by a different purchase")
		}
		if order.Status != models.OrderStatusPending {
			return fmt.Errorf("order is not pending (status=%s)", order.Status)
		}
		if order.PackageID == nil && order.ProductID == nil {
			return fmt.Errorf("order has neither package nor product")
		}

		// Reject a token already tied to a different order.
		var reused models.Payment
		if err := tx.Where("transaction_id = ? AND order_id != ?", purchaseToken, order.ID).First(&reused).Error; err == nil {
			return ErrPlayPurchaseTokenReused
		}

		var providerOrderID string
		var expiresAt *time.Time
		var playProductID string
		var isSubscription bool

		if order.PackageID != nil {
			pkg, err := models.FindPremiumPackageByID(tx, order.PackageID.String())
			if err != nil {
				return fmt.Errorf("load package: %w", err)
			}
			if pkg.PlayProductID == nil || *pkg.PlayProductID == "" {
				return ErrPackageNotPlayMapped
			}

			valid, pID, expiry, err := s.verifyWithPlay(ctx, pkg, purchaseToken)
			if err != nil {
				return fmt.Errorf("verify with google play: %w", err)
			}
			if !valid {
				return ErrPlayPurchaseNotValid
			}
			providerOrderID, expiresAt, playProductID = pID, expiry, *pkg.PlayProductID
			isSubscription = pkg.Type == "subscription"
		} else {
			product, err := models.FindProductByID(tx, *order.ProductID)
			if err != nil {
				return fmt.Errorf("load product: %w", err)
			}
			if product.PlayProductID == nil || *product.PlayProductID == "" {
				return ErrProductNotPlayMapped
			}

			valid, pID, err := s.verifyProductWithPlay(ctx, product, purchaseToken)
			if err != nil {
				return fmt.Errorf("verify with google play: %w", err)
			}
			if !valid {
				return ErrPlayPurchaseNotValid
			}
			providerOrderID, playProductID = pID, *product.PlayProductID
		}

		rawPayload, _ := json.Marshal(map[string]string{"purchase_token": purchaseToken, "product_id": playProductID})
		payment := models.Payment{
			OrderID:           order.ID,
			ProviderOrderID:   providerOrderID,
			UserID:            &order.UserID,
			TransactionID:     purchaseToken,
			TransactionStatus: "settlement",
			PaymentType:       "google_play",
			GrossAmount:       fmt.Sprintf("%d.00", order.AmountIdr),
			RawPayload:        string(rawPayload),
		}
		if err := tx.Create(&payment).Error; err != nil {
			return fmt.Errorf("save payment: %w", err)
		}

		if err := tx.Model(&order).Update("status", models.OrderStatusPaid).Error; err != nil {
			return fmt.Errorf("update order status: %w", err)
		}
		order.Status = models.OrderStatusPaid
		result = order

		if err := s.entitlementService.GrantForPaidOrder(tx, &order); err != nil {
			return fmt.Errorf("grant entitlements: %w", err)
		}
		if isSubscription && expiresAt != nil {
			if err := s.entitlementService.SyncSubscriptionExpiry(tx, order.UserID, *order.PackageID, *expiresAt); err != nil {
				return fmt.Errorf("sync subscription expiry: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Google Play Real-time Developer Notifications ───────────────────────────

type playDeveloperNotification struct {
	PackageName              string                        `json:"packageName"`
	SubscriptionNotification *playSubscriptionNotification `json:"subscriptionNotification"`
}

type playSubscriptionNotification struct {
	NotificationType int    `json:"notificationType"`
	PurchaseToken    string `json:"purchaseToken"`
	SubscriptionID   string `json:"subscriptionId"`
}

// Google Play RTDN subscriptionNotification.notificationType values we act
// on; see https://developer.android.com/google/play/billing/rtdn-reference.
const (
	playNotifSubscriptionRecovered = 1
	playNotifSubscriptionRenewed   = 2
	playNotifSubscriptionPurchased = 4
	playNotifSubscriptionRestarted = 7
	playNotifSubscriptionRevoked   = 12
	playNotifSubscriptionExpired   = 13
)

// HandlePlayRTDN processes a decoded Google Play Real-time Developer
// Notification payload (the base64-decoded Pub/Sub message data) and
// reconciles the affected subscription's entitlement. A notification for a
// purchase token with no matching payment (a test notification, or one
// predating this endpoint) is ignored rather than erroring — RTDN delivery
// is at-least-once and best-effort by nature.
func (s *PaymentService) HandlePlayRTDN(ctx context.Context, raw []byte) error {
	var notif playDeveloperNotification
	if err := json.Unmarshal(raw, &notif); err != nil {
		return fmt.Errorf("decode notification: %w", err)
	}
	sub := notif.SubscriptionNotification
	if sub == nil || sub.PurchaseToken == "" {
		return nil
	}

	var payment models.Payment
	if err := s.db.Where("transaction_id = ? AND payment_type = ?", sub.PurchaseToken, "google_play").First(&payment).Error; err != nil {
		return nil
	}
	order, err := models.FindOrderByID(s.db, payment.OrderID)
	if err != nil || order.PackageID == nil {
		return nil
	}

	switch sub.NotificationType {
	case playNotifSubscriptionRevoked, playNotifSubscriptionExpired:
		return s.entitlementService.RevokeSubscription(order.UserID)
	case playNotifSubscriptionRecovered, playNotifSubscriptionRenewed, playNotifSubscriptionRestarted, playNotifSubscriptionPurchased:
		result, err := s.playVerifier.VerifySubscriptionPurchase(ctx, sub.SubscriptionID, sub.PurchaseToken)
		if err != nil {
			return err
		}
		if result.ExpiryTimeMillis == "" {
			return nil
		}
		millis, err := strconv.ParseInt(result.ExpiryTimeMillis, 10, 64)
		if err != nil {
			return fmt.Errorf("parse expiryTimeMillis: %w", err)
		}
		return s.entitlementService.SyncSubscriptionExpiry(s.db, order.UserID, *order.PackageID, time.UnixMilli(millis))
	default:
		// Cancellation (not yet expired), hold, grace period, price-change,
		// pause, etc. — no immediate entitlement change required.
		return nil
	}
}

// verifyWithPlay checks the purchase against the Android Publisher API,
// dispatching to the product or subscription endpoint based on the
// package's type. Returns the Play-reported order id and, for
// subscriptions, the current expiry.
func (s *PaymentService) verifyWithPlay(ctx context.Context, pkg *models.PremiumPackage, purchaseToken string) (valid bool, providerOrderID string, expiresAt *time.Time, err error) {
	if pkg.Type == "subscription" {
		sub, err := s.playVerifier.VerifySubscriptionPurchase(ctx, *pkg.PlayProductID, purchaseToken)
		if err != nil {
			return false, "", nil, err
		}
		if sub.ExpiryTimeMillis == "" {
			return false, "", nil, nil
		}
		millis, err := strconv.ParseInt(sub.ExpiryTimeMillis, 10, 64)
		if err != nil {
			return false, "", nil, fmt.Errorf("parse expiryTimeMillis: %w", err)
		}
		expiry := time.UnixMilli(millis)
		if !expiry.After(time.Now()) {
			return false, sub.OrderID, nil, nil
		}
		return true, sub.OrderID, &expiry, nil
	}

	product, err := s.playVerifier.VerifyProductPurchase(ctx, *pkg.PlayProductID, purchaseToken)
	if err != nil {
		return false, "", nil, err
	}
	if product.PurchaseState != PlayPurchaseStatePurchased {
		return false, product.OrderID, nil, nil
	}
	return true, product.OrderID, nil, nil
}

// verifyProductWithPlay checks a product purchase against the Android Publisher API.
func (s *PaymentService) verifyProductWithPlay(ctx context.Context, product *models.Product, purchaseToken string) (valid bool, providerOrderID string, err error) {
	playProduct, err := s.playVerifier.VerifyProductPurchase(ctx, *product.PlayProductID, purchaseToken)
	if err != nil {
		return false, "", err
	}
	if playProduct.PurchaseState != PlayPurchaseStatePurchased {
		return false, playProduct.OrderID, nil
	}
	return true, playProduct.OrderID, nil
}

// ── Google Play User Choice Billing — alternative billing reporting ────────
//
// Under User Choice Billing, Google Play lets the user pick Arunika's own
// billing (Midtrans) instead of Google Play Billing. The purchase itself is
// then just an ordinary Midtrans transaction — CreateSnapTransaction and the
// webhook already settle the order and grant entitlement exactly as they do
// for any other Midtrans purchase. What's specific to User Choice Billing is
// this: Google still requires the transaction to be *reported* to them
// (Play calculates a reduced service fee off of it), which is what this
// does once the Midtrans order is confirmed PAID.

// ErrOrderNotPaid is returned when reporting is attempted for an order that
// hasn't settled yet.
var ErrOrderNotPaid = fmt.Errorf("order is not paid")

// ReportExternalTransaction reports a Midtrans-settled order to Google as a
// User Choice Billing external transaction. Idempotent: a second call for
// an already-reported order is a no-op.
func (s *PaymentService) ReportExternalTransaction(ctx context.Context, orderID, userID uuid.UUID, externalTransactionToken string) (*models.Order, error) {
	order, err := models.FindOrderByID(s.db, orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}
	if order.UserID != userID {
		return nil, fmt.Errorf("order not found: %w", gorm.ErrRecordNotFound)
	}
	if order.Status != models.OrderStatusPaid {
		return nil, ErrOrderNotPaid
	}
	if order.PackageID == nil {
		return nil, fmt.Errorf("order has no package")
	}

	var alreadyReported models.Payment
	if err := s.db.Where("order_id = ? AND payment_type = ?", order.ID, "google_play_external_report").
		First(&alreadyReported).Error; err == nil {
		return order, nil
	}

	pkg, err := models.FindPremiumPackageByID(s.db, order.PackageID.String())
	if err != nil {
		return nil, fmt.Errorf("load package: %w", err)
	}

	amountMicros := strconv.FormatInt(order.AmountIdr*1_000_000, 10)
	reportReq := CreateExternalTransactionRequest{
		OriginalPreTaxAmount: ExternalTransactionPrice{Currency: "IDR", PriceMicros: amountMicros},
		OriginalTaxAmount:    ExternalTransactionPrice{Currency: "IDR", PriceMicros: "0"},
		TransactionTime:      time.Now().UTC().Format(time.RFC3339),
		UserTaxAddress:       ExternalTransactionAddress{RegionCode: "ID"},
	}
	if pkg.Type == "subscription" {
		reportReq.RecurringTransaction = &RecurringExternalTransaction{
			ExternalTransactionToken: externalTransactionToken,
			ExternalSubscription:     ExternalSubscription{SubscriptionType: "RECURRING"},
		}
	} else {
		reportReq.OneTimeTransaction = &OneTimeExternalTransaction{ExternalTransactionToken: externalTransactionToken}
	}

	if err := s.playVerifier.ReportExternalTransaction(ctx, order.ID.String(), reportReq); err != nil {
		return nil, fmt.Errorf("report to google play: %w", err)
	}

	payment := models.Payment{
		OrderID:           order.ID,
		ProviderOrderID:   "extxn-" + order.ID.String(),
		UserID:            &order.UserID,
		TransactionID:     externalTransactionToken,
		TransactionStatus: "reported",
		PaymentType:       "google_play_external_report",
		GrossAmount:       fmt.Sprintf("%d.00", order.AmountIdr),
	}
	if err := s.db.Create(&payment).Error; err != nil {
		return nil, fmt.Errorf("save report audit row: %w", err)
	}

	return order, nil
}
