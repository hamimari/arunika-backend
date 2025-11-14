package models

import (
	"time"
)

type Voucher struct {
	Code          string    `json:"code"`
	DiscountType  string    `json:"discount_type"`
	DiscountValue float32   `json:"discount_value"`
	MaxUses       int64     `json:"max_uses"`
	ExpiredAt     time.Time `json:"expired_at"`
}
