package models

import (
	"time"
)

type Voucher struct {
	Code          string    `json:"code"`
	DiscountType  string    `json:"discount_type"`
	DiscountValue float32   `json:"discount_value"`
	PlanID        *uint     `json:"plan_id"`
	FeatureID     *uint     `json:"feature_id"`
	MaxUses       int64     `json:"max_uses"`
	ExpiredAt     time.Time `json:"expired_at"`
}
