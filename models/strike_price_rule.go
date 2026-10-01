package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	StrikeScopeArCard  = "AR_CARD"
	StrikeScopeDongeng = "DONGENG"
	StrikeScopePackage = "PACKAGE"

	StrikeModeNone    = "NONE"
	StrikeModePercent = "PERCENT"
	StrikeModeFixed   = "FIXED"
)

// StrikePriceRule is the global promotional strike-price rule for one scope
// (see V56). A rule only produces a strike price while inside its
// [StartsAt, EndsAt) period.
type StrikePriceRule struct {
	Scope     string     `gorm:"column:scope;primaryKey" json:"scope"`
	Mode      string     `gorm:"column:mode;not null"    json:"mode"`
	Value     int        `gorm:"column:value;not null"   json:"value"`
	StartsAt  *time.Time `gorm:"column:starts_at"        json:"starts_at"`
	EndsAt    *time.Time `gorm:"column:ends_at"          json:"ends_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at"       json:"updated_at"`
}

func (StrikePriceRule) TableName() string { return "strike_price_rules" }

// StrikeOverride is the per-item strike-price override embedded in Product
// and PremiumPackage. A nil StrikeMode means the item inherits its scope's
// global rule.
type StrikeOverride struct {
	StrikeMode     *string    `gorm:"column:strike_mode"      json:"strike_mode,omitempty"`
	StrikeValue    *int       `gorm:"column:strike_value"     json:"strike_value,omitempty"`
	StrikeStartsAt *time.Time `gorm:"column:strike_starts_at" json:"strike_starts_at,omitempty"`
	StrikeEndsAt   *time.Time `gorm:"column:strike_ends_at"   json:"strike_ends_at,omitempty"`
}

// StrikeDisplay is the computed, display-only strike price for an item.
// All three fields are nil together when there is no active promo.
type StrikeDisplay struct {
	StrikePriceIdr  *int64     `gorm:"-" json:"strike_price_idr"`
	DiscountPercent *int       `gorm:"-" json:"discount_percent"`
	PromoEndsAt     *time.Time `gorm:"-" json:"promo_ends_at"`
}

func FindAllStrikePriceRules(db *gorm.DB) ([]StrikePriceRule, error) {
	var rules []StrikePriceRule
	err := db.Order("scope ASC").Find(&rules).Error
	return rules, err
}
