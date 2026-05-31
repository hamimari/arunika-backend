package services

import (
	"arunika_backend/models"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MidtransSnapResponse is the response from the Midtrans Snap API.
type MidtransSnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

// MidtransNotification is the webhook payload from Midtrans.
type MidtransNotification struct {
	OrderID           string `json:"order_id"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	PaymentType       string `json:"payment_type"`
	GrossAmount       string `json:"gross_amount"`
}

type PaymentService struct {
	db *gorm.DB
}

func NewPaymentService(db *gorm.DB) *PaymentService {
	return &PaymentService{db: db}
}

// CreateSnapToken calls Midtrans Snap API and stores the transaction.
func (s *PaymentService) CreateSnapToken(userID, planName string, amount int64) (*models.PaymentTransaction, error) {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	snapURL := os.Getenv("MIDTRANS_SNAP_URL")
	if snapURL == "" {
		snapURL = "https://app.sandbox.midtrans.com/snap/v1/transactions"
	}

	orderID := fmt.Sprintf("order-%s", uuid.New().String())

	payload := map[string]interface{}{
		"transaction_details": map[string]interface{}{
			"order_id":     orderID,
			"gross_amount": amount,
		},
		"item_details": []map[string]interface{}{
			{
				"id":       "plan-1",
				"price":    amount,
				"quantity": 1,
				"name":     planName,
			},
		},
		"customer_details": map[string]interface{}{
			"customer_id": userID,
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", snapURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create snap request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(serverKey + ":"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("snap api call: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("midtrans error %d: %s", resp.StatusCode, string(respBody))
	}

	var snapResp MidtransSnapResponse
	if err := json.Unmarshal(respBody, &snapResp); err != nil {
		return nil, fmt.Errorf("parse snap response: %w", err)
	}

	tx := &models.PaymentTransaction{
		UserID:    userID,
		OrderID:   orderID,
		PlanName:  planName,
		Amount:    amount,
		SnapToken: snapResp.Token,
		Status:    "pending",
	}

	if err := models.CreatePaymentTransaction(s.db, tx); err != nil {
		return nil, fmt.Errorf("save transaction: %w", err)
	}

	return tx, nil
}

// HandleNotification processes a Midtrans webhook and updates DB.
func (s *PaymentService) HandleNotification(notification MidtransNotification) error {
	var (
		status = "pending"
		paidAt *time.Time
	)

	switch notification.TransactionStatus {
	case "capture", "settlement":
		if notification.FraudStatus == "" || notification.FraudStatus == "accept" {
			status = "success"
			now := time.Now()
			paidAt = &now
		} else {
			status = "failed"
		}
	case "deny", "cancel", "expire":
		status = "failed"
	case "pending":
		status = "pending"
	}

	return models.UpdatePaymentTransactionStatus(
		s.db,
		notification.OrderID,
		status,
		notification.TransactionStatus,
		notification.PaymentType,
		paidAt,
	)
}
