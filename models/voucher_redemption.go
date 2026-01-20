package models

import (
	"gorm.io/gorm"
	"time"
)

type VoucherRedemption struct {
	gorm.Model
	VoucherId string    `json:"voucher_id"`
	ParentId  string    `json:"parent_id"`
	UsedAt    time.Time `json:"used_at"`
}
