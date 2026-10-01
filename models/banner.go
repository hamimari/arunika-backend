package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type Banner struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Title       string    `gorm:"not null"                                        json:"title"`
	ImageURL    string    `gorm:"column:image_url;not null"                       json:"image_url"`
	LinkURL     string    `gorm:"column:link_url"                                 json:"link_url"`
	Description string    `gorm:"column:description"                              json:"description"`
	IsActive    bool      `gorm:"column:is_active;default:true"                   json:"is_active"`
	SortOrder   int       `gorm:"column:sort_order;default:0"                     json:"sort_order"`
	Hidden      bool      `gorm:"column:hidden;default:false"                     json:"hidden"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsDeleted   bool      `gorm:"column:is_deleted;default:false"                 json:"-"`
}

func (Banner) TableName() string { return "banners" }

// FindActiveBanners returns all visible, active banners ordered by sort_order.
// Used by the mobile app home endpoint.
func FindActiveBanners(db *gorm.DB) ([]Banner, error) {
	var banners []Banner
	err := db.Where("is_deleted = false AND hidden = false AND is_active = true").
		Order("sort_order ASC").Find(&banners).Error
	return banners, err
}
