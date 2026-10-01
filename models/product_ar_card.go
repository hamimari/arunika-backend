package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProductArCard maps a Product to the single AR card it unlocks.
type ProductArCard struct {
	ProductID uuid.UUID `gorm:"column:product_id;type:uuid;primaryKey"           json:"product_id"`
	ArCardID  string    `gorm:"column:ar_card_id;type:uuid;not null;uniqueIndex" json:"ar_card_id"`
}

func (ProductArCard) TableName() string { return "product_ar_cards" }

// FindProductByArCardID returns the product linked to arCardID, or nil if the
// card has no product (i.e. it's free).
func FindProductByArCardID(db *gorm.DB, arCardID string) (*Product, error) {
	var mapping ProductArCard
	if err := db.Where("ar_card_id = ?", arCardID).First(&mapping).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return FindProductByID(db, mapping.ProductID)
}
