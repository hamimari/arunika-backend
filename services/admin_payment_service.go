package services

import (
	"arunika_backend/models"
	"gorm.io/gorm"
)

type AdminPaymentService struct {
	db *gorm.DB
}

func NewAdminPaymentService(db *gorm.DB) *AdminPaymentService {
	return &AdminPaymentService{db: db}
}

// List returns a paginated, filterable list of payment transactions.
func (s *AdminPaymentService) List(status, search string, page, perPage int) ([]models.PaymentTransaction, int64, error) {
	var items []models.PaymentTransaction
	var total int64

	q := s.db.Model(&models.PaymentTransaction{})
	if status != "" {
		q = q.Where("transaction_status = ?", status)
	}
	if search != "" {
		q = q.Where("order_id ILIKE ? OR transaction_id ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	q.Count(&total)
	err := q.Order("created_at DESC").
		Limit(perPage).Offset((page - 1) * perPage).
		Find(&items).Error
	return items, total, err
}

// Get returns a single payment transaction by ID.
func (s *AdminPaymentService) Get(id string) (*models.PaymentTransaction, error) {
	var item models.PaymentTransaction
	err := s.db.Where("id = ?", id).First(&item).Error
	return &item, err
}
