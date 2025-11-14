package models

import (
	"gorm.io/gorm"
	"time"
)

type VoucherUsage struct {
	gorm.Model
	VoucherId      string    `json:"voucher_id"`
	SubscriptionId string    `json:"subscription_id"`
	ParentId       string    `json:"parent_id"`
	UsedAt         time.Time `json:"used_at"`
}
