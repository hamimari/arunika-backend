package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Banner struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Title     string    `gorm:"type:varchar(200);not null;default:''"          json:"title"`
	ImageURL  string    `gorm:"column:image_url;type:text;not null;default:''" json:"image_url"`
	Type      string    `gorm:"type:varchar(50);not null;default:'promo'"      json:"type"`
	IsActive  bool      `gorm:"column:is_active;not null;default:true"         json:"is_active"`
	SortOrder int       `gorm:"column:sort_order;not null;default:0"           json:"sort_order"`
	CtaURL    *string   `gorm:"column:cta_url;type:text"                       json:"cta_url,omitempty"`
	Emoji     *string   `gorm:"type:varchar(20)"                               json:"emoji,omitempty"`
	Fact      *string   `gorm:"type:text"                                      json:"fact,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsDeleted bool      `gorm:"column:is_deleted;not null;default:false"       json:"-"`
}

func FindActiveBanners(db *gorm.DB) ([]Banner, error) {
	var banners []Banner
	err := db.
		Where("is_active = ? AND is_deleted = ?", true, false).
		Order("sort_order ASC").
		Find(&banners).Error
	if err != nil {
		return nil, err
	}
	return banners, nil
}
