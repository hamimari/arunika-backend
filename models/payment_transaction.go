package models

import (
	"gorm.io/gorm"
	"time"
)

// PaymentTransaction records a Midtrans payment attempt.
type PaymentTransaction struct {
	ID             string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID         string     `gorm:"type:uuid;not null"                             json:"user_id"`
	OrderID        string     `gorm:"type:varchar(100);uniqueIndex;not null"         json:"order_id"`
	PlanName       string     `gorm:"type:varchar(255);not null"                     json:"plan_name"`
	Amount         int64      `gorm:"not null"                                       json:"amount"`
	SnapToken      string     `gorm:"type:text;not null;default:''"                  json:"snap_token"`
	PaymentType    string     `gorm:"type:varchar(100);not null;default:''"          json:"payment_type"`
	Status         string     `gorm:"type:varchar(50);not null;default:'pending'"    json:"status"`
	MidtransStatus string     `gorm:"type:varchar(50);not null;default:''"           json:"midtrans_status"`
	PaidAt         *time.Time `gorm:"default:null"                                   json:"paid_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	IsDeleted      bool       `gorm:"not null;default:false"                         json:"is_deleted"`
}

func (PaymentTransaction) TableName() string {
	return "payment_transactions"
}

func CreatePaymentTransaction(db *gorm.DB, tx *PaymentTransaction) error {
	return db.Create(tx).Error
}

func FindPaymentTransactionByOrderID(db *gorm.DB, orderID string) (*PaymentTransaction, error) {
	var tx PaymentTransaction
	result := db.Where("order_id = ?", orderID).First(&tx)
	return &tx, result.Error
}

func UpdatePaymentTransactionStatus(db *gorm.DB, orderID, status, midtransStatus, paymentType string, paidAt *time.Time) error {
	updates := map[string]interface{}{
		"status":          status,
		"midtrans_status": midtransStatus,
		"payment_type":    paymentType,
		"updated_at":      time.Now(),
	}
	if paidAt != nil {
		updates["paid_at"] = paidAt
	}
	return db.Model(&PaymentTransaction{}).Where("order_id = ?", orderID).Updates(updates).Error
}
