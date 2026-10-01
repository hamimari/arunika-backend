package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PremiumPackageItem maps a premium package to one of the products it bundles.
type PremiumPackageItem struct {
	PackageID uuid.UUID `gorm:"column:package_id;type:uuid;primaryKey" json:"package_id"`
	ProductID uuid.UUID `gorm:"column:product_id;type:uuid;primaryKey" json:"product_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (PremiumPackageItem) TableName() string { return "premium_package_items" }

// FindPackageItems returns every product bundled in a package.
func FindPackageItems(db *gorm.DB, packageID uuid.UUID) ([]PremiumPackageItem, error) {
	var items []PremiumPackageItem
	result := db.Where("package_id = ?", packageID).Find(&items)
	return items, result.Error
}

// AddPackageItem bundles a product into a package. Idempotent — re-adding an
// already-bundled product is a no-op.
func AddPackageItem(db *gorm.DB, packageID, productID uuid.UUID) error {
	return db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&PremiumPackageItem{PackageID: packageID, ProductID: productID}).Error
}

// RemovePackageItem un-bundles a product from a package.
func RemovePackageItem(db *gorm.DB, packageID, productID uuid.UUID) error {
	return db.Where("package_id = ? AND product_id = ?", packageID, productID).Delete(&PremiumPackageItem{}).Error
}
