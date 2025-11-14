package models

import (
	"time"
)

type Subscription struct {
	BaseModel
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Status    string    `json:"status"`
	AutoRenew bool      `json:"auto_renew"`
}
