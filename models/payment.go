package models

import (
	"github.com/google/uuid"
	"time"
)

// Payment records every Midtrans webhook callback received for an Order.
// One row per callback — so multiple status transitions for the same order are all stored.
type Payment struct {
	ID                uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"     json:"id"`
	OrderID           uuid.UUID  `gorm:"column:order_id;type:uuid;not null"                   json:"order_id"`
	ProviderOrderID   string     `gorm:"column:provider_order_id;not null"                    json:"provider_order_id"`
	UserID            *uuid.UUID `gorm:"column:user_id"                                       json:"user_id"`
	TransactionID     string     `gorm:"column:transaction_id"                                json:"transaction_id"`
	TransactionStatus string     `gorm:"column:transaction_status;not null"                   json:"transaction_status"`
	PaymentType       string     `gorm:"column:payment_type"                                  json:"payment_type"`
	GrossAmount       string     `gorm:"column:gross_amount"                                  json:"gross_amount"`
	StatusCode        string     `gorm:"column:status_code"                                   json:"status_code"`
	FraudStatus       string     `gorm:"column:fraud_status"                                  json:"fraud_status"`
	PaymentURL        string     `gorm:"column:payment_url"                                   json:"payment_url,omitempty"`
	RawPayload        string     `gorm:"column:raw_payload;type:jsonb;not null;default:'{}'" json:"raw_payload"`
	CreatedAt         time.Time  `gorm:"column:created_at"                                    json:"created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"                                    json:"updated_at"`
}

func (Payment) TableName() string { return "payments" }
