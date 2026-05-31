package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ArCardCategory represents a hierarchical category for AR cards.
// Top-level categories have ParentID == nil.
// Sub-categories have ParentID pointing to their parent.
type ArCardCategory struct {
	ID        uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string           `gorm:"type:varchar(100);not null"                     json:"name"`
	Emoji     string           `gorm:"type:varchar(20);not null;default:''"           json:"emoji"`
	ImageURL  string           `gorm:"column:image_url;type:text;default:''"          json:"image_url"`
	ParentID  *uuid.UUID       `gorm:"column:parent_id;type:uuid"                     json:"parent_id,omitempty"`
	SortOrder int              `gorm:"column:sort_order;not null;default:0"           json:"sort_order"`
	Children  []ArCardCategory `gorm:"foreignKey:ParentID"                           json:"children,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	IsDeleted bool             `gorm:"column:is_deleted;not null;default:false"       json:"-"`
}

// FindTopLevelCategories returns all non-deleted top-level categories
// (ParentID IS NULL) with their sub-categories preloaded.
func FindTopLevelCategories(db *gorm.DB) ([]ArCardCategory, error) {
	var cats []ArCardCategory
	err := db.
		Where("parent_id IS NULL AND is_deleted = ?", false).
		Order("sort_order ASC").
		Preload("Children", "is_deleted = ?", false).
		Find(&cats).Error
	if err != nil {
		return nil, err
	}
	return cats, nil
}
