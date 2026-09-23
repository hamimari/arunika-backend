package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Product is one purchasable content item, linked to a Feature (AR_CARD or DONGENG).
// The actual content item it unlocks is found via ProductArCard/ProductDongeng.
type Product struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	FeatureID     uuid.UUID `gorm:"column:feature_id;type:uuid;not null"           json:"feature_id"`
	PriceIdr      int64     `gorm:"column:price_idr;not null"                      json:"price_idr"`
	PlayProductID *string   `gorm:"column:play_product_id;type:text"               json:"play_product_id"`
	IsActive      bool      `gorm:"column:is_active;not null;default:true"         json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Product) TableName() string { return "products" }

func FindProductByID(db *gorm.DB, id uuid.UUID) (*Product, error) {
	var p Product
	if err := db.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func FindAllProducts(db *gorm.DB) ([]Product, error) {
	var products []Product
	result := db.Order("created_at desc").Find(&products)
	return products, result.Error
}
