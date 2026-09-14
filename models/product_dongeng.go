package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProductDongeng maps a Product to the single dongeng it unlocks.
type ProductDongeng struct {
	ProductID uuid.UUID `gorm:"column:product_id;type:uuid;primaryKey"           json:"product_id"`
	DongengID uuid.UUID `gorm:"column:dongeng_id;type:uuid;not null;uniqueIndex" json:"dongeng_id"`
}

func (ProductDongeng) TableName() string { return "product_dongengs" }

// FindProductByDongengID returns the product linked to dongengID, or nil if
// the dongeng has no product (i.e. it's free).
func FindProductByDongengID(db *gorm.DB, dongengID uuid.UUID) (*Product, error) {
	var mapping ProductDongeng
	if err := db.Where("dongeng_id = ?", dongengID).First(&mapping).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return FindProductByID(db, mapping.ProductID)
}
