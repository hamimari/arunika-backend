package services

import (
	"arunika_backend/models"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentService struct {
	db                 *gorm.DB
	entitlementService *EntitlementService
}

func NewPaymentService(db *gorm.DB, entitlementService *EntitlementService) *PaymentService {
	return &PaymentService{db: db, entitlementService: entitlementService}
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
