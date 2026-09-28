package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	RefundSourceAdmin        = "ADMIN"
	RefundSourceGoogleVoided = "GOOGLE_VOIDED"
	RefundSourceGoogleRTDN   = "GOOGLE_RTDN"

	RefundTypeFull     = "FULL"
	RefundTypeProrated = "PRORATED"

	RefundStatusRequested = "REQUESTED"
	RefundStatusSucceeded = "SUCCEEDED"
	RefundStatusFailed    = "FAILED"
)

// OrderRefund records one refund of a Google Play order (V59) — issued from
// the backoffice or reported by Google — with the amounts Google reports.
type OrderRefund struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrderID          uuid.UUID  `gorm:"column:order_id;type:uuid;not null"              json:"order_id"`
	Provider         string     `gorm:"column:provider;not null;default:google_play"    json:"provider"`
	Source           string     `gorm:"column:source;not null"                          json:"source"`
	RefundType       string     `gorm:"column:refund_type;not null;default:FULL"        json:"refund_type"`
	Revoked          bool       `gorm:"column:revoked;not null;default:true"            json:"revoked"`
	Reason           *string    `gorm:"column:reason"                                   json:"reason"`
	AdminID          *uuid.UUID `gorm:"column:admin_id;type:uuid"                       json:"admin_id"`
	PlayOrderID      *string    `gorm:"column:play_order_id"                            json:"play_order_id"`
	PurchaseToken    *string    `gorm:"column:purchase_token"                           json:"-"`
	OrderAmountIdr   int64      `gorm:"column:order_amount_idr;not null"                json:"order_amount_idr"`
	RefundedTotal    *float64   `gorm:"column:refunded_total"                           json:"refunded_total"`
	RefundedTax      *float64   `gorm:"column:refunded_tax"                             json:"refunded_tax"`
	Currency         *string    `gorm:"column:currency"                                 json:"currency"`
	PlayOrderState   *string    `gorm:"column:play_order_state"                         json:"play_order_state"`
	PlayRefundReason *string    `gorm:"column:play_refund_reason"                       json:"play_refund_reason"`
	VoidedSource     *int       `gorm:"column:voided_source"                            json:"voided_source"`
	VoidedReason     *int       `gorm:"column:voided_reason"                            json:"voided_reason"`
	Status           string     `gorm:"column:status;not null"                          json:"status"`
	Error            *string    `gorm:"column:error"                                    json:"error"`
	RawResponse      *string    `gorm:"column:raw_response;type:jsonb"                  json:"-"`
	RequestedAt      time.Time  `gorm:"column:requested_at"                             json:"requested_at"`
	CompletedAt      *time.Time `gorm:"column:completed_at"                             json:"completed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (OrderRefund) TableName() string { return "order_refunds" }
