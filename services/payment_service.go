package services

import (
	"arunika_backend/models"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

type PaymentService struct {
	db *gorm.DB
}

func NewPaymentService(db *gorm.DB) *PaymentService {
	return &PaymentService{db: db}
}

type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

// CreateSnapTransaction creates a Midtrans Snap transaction for a monthly subscription.
func (s *PaymentService) CreateSnapTransaction(userID uuid.UUID) (*SnapResponse, error) {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		return nil, fmt.Errorf("MIDTRANS_SERVER_KEY not configured")
	}

	orderID := fmt.Sprintf("sub-%s-%d", userID.String(), time.Now().UnixMilli())
	grossAmount := 49000 // IDR 49,000 / month

	payload := map[string]interface{}{
		"transaction_details": map[string]interface{}{
			"order_id":     orderID,
			"gross_amount": grossAmount,
		},
		"customer_details": map[string]interface{}{
			"email": "",
		},
		"item_details": []map[string]interface{}{
			{
				"id":       "edu-premium-monthly",
				"price":    grossAmount,
				"quantity": 1,
				"name":     "Arunika Premium Bulanan",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://app.sandbox.midtrans.com/snap/v1/transactions", strings.NewReader(string(body)))
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

	// Store the pending order ID in user_subscriptions for later webhook matching.
	if err := s.db.Model(&models.UserSubscription{}).
		Where("user_id = ?", userID).
		Update("midtrans_order_id", orderID).Error; err != nil {
		slog.Warn("PaymentService: failed to store order_id", "error", err)
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

// HandleWebhook processes a Midtrans payment notification.
// Returns userID so the caller can dispatch a push notification.
func (s *PaymentService) HandleWebhook(notif WebhookNotification) (uuid.UUID, error) {
	if !ValidateWebhookSignature(notif.OrderID, notif.StatusCode, notif.GrossAmount, notif.SignatureKey) {
		return uuid.Nil, fmt.Errorf("invalid signature")
	}

	// Persist every callback regardless of status — gives a full audit trail.
	s.saveTransaction(notif)

	// Only activate premium on settlement.
	settled := (notif.TransactionStatus == "settlement") ||
		(notif.TransactionStatus == "capture" && notif.FraudStatus == "accept")
	if !settled {
		return uuid.Nil, nil // not an error, just not settled yet
	}

	expiresAt := time.Now().AddDate(0, 1, 0) // +1 month
	result := s.db.Model(&models.UserSubscription{}).
		Where("midtrans_order_id = ?", notif.OrderID).
		Updates(map[string]interface{}{
			"status":     "premium",
			"expires_at": expiresAt,
		})
	if result.Error != nil {
		return uuid.Nil, fmt.Errorf("update subscription: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return uuid.Nil, fmt.Errorf("subscription not found for order %s", notif.OrderID)
	}

	// Fetch the userID for notification dispatch.
	var sub models.UserSubscription
	if err := s.db.Where("midtrans_order_id = ?", notif.OrderID).First(&sub).Error; err != nil {
		return uuid.Nil, nil // subscription updated but we can't find userID; non-fatal
	}
	return sub.UserID, nil
}

// saveTransaction writes the raw webhook notification to payment_transactions.
func (s *PaymentService) saveTransaction(notif WebhookNotification) {
	rawPayload, _ := json.Marshal(notif)

	tx := models.PaymentTransaction{
		OrderID:           notif.OrderID,
		TransactionID:     notif.TransactionID,
		TransactionStatus: notif.TransactionStatus,
		PaymentType:       notif.PaymentType,
		GrossAmount:       notif.GrossAmount,
		StatusCode:        notif.StatusCode,
		FraudStatus:       notif.FraudStatus,
		RawPayload:        string(rawPayload),
	}

	// Resolve user_id from user_subscriptions via the order_id.
	var sub models.UserSubscription
	if err := s.db.Select("user_id").Where("midtrans_order_id = ?", notif.OrderID).First(&sub).Error; err == nil {
		tx.UserID = &sub.UserID
	}

	if err := s.db.Create(&tx).Error; err != nil {
		slog.Warn("PaymentService: failed to save payment transaction", "error", err)
	}
}

// GetSubscription returns the subscription record for a user.
func (s *PaymentService) GetSubscription(userID uuid.UUID) (*models.UserSubscription, error) {
	var sub models.UserSubscription
	if err := s.db.Where("user_id = ?", userID).First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}
