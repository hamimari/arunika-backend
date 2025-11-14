package models

import (
	"math/big"
)

type SubscriptionPlan struct {
	BaseModel
	Name         string  `json:"name"`
	Price        big.Int `json:"price"`
	DurationDays int     `json:"duration_days"`
	IsActive     bool    `json:"is_active"`
}
