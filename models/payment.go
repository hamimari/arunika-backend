package models

import (
	"math/big"
	"time"
)

type Payment struct {
	BaseModel
	SubscriptionId string    `json:"subscription_id"`
	Amount         big.Int   `json:"amount"`
	PaymentDate    time.Time `json:"payment_date"`
	PaymentMethod  string    `json:"payment_method"`
	Status         string    `json:"status"`
}
