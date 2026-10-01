package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Dongeng struct {
	BaseModel
	Title      string        `json:"title"       gorm:"column:title"`
	AgeStart   float32       `json:"age_start"   gorm:"column:age_start"`
	AgeEnd     float32       `json:"age_end"     gorm:"column:age_end"`
	ImageUrl   string        `json:"image_url"   gorm:"column:image_url"`
	AudioUrl   string        `json:"audio_url"   gorm:"column:audio_url"`
	IsFree     bool          `json:"is_free"     gorm:"column:is_free"`
	CategoryId *uuid.UUID    `json:"category_id" gorm:"column:category_id;type:uuid"` // nullable: empty must be NULL, not ''
	Duration   int64         `json:"duration"    gorm:"column:duration"`
	Hidden     bool          `json:"hidden"      gorm:"column:hidden;default:false"`
	Pages      []DongengPage `json:"pages"       gorm:"foreignKey:DongengId"`
	// Structured category FKs, additive alongside the legacy free-text
	// CategoryId above (which points at the unrelated generic `categories`
	// table and is left untouched by this).
	DongengCategoryID    *uuid.UUID       `gorm:"column:dongeng_category_id;type:uuid"     json:"dongeng_category_id,omitempty"`
	DongengSubCategoryID *uuid.UUID       `gorm:"column:dongeng_sub_category_id;type:uuid" json:"dongeng_sub_category_id,omitempty"`
	CategoryRef          *DongengCategory `gorm:"foreignKey:DongengCategoryID"             json:"category_ref,omitempty"`
	SubCategoryRef       *DongengCategory `gorm:"foreignKey:DongengSubCategoryID"          json:"sub_category_ref,omitempty"`
}

// FindAllFairyTales returns a paginated, optionally-filtered list of dongengs
// and the total matching count (for the caller to derive hasMore).
// Deleted and hidden (backoffice visibility toggle) dongengs are excluded.
// search is case-insensitive title prefix/substring match; empty string = no filter.
// categoryID/subCategoryID, when non-empty, filter to that dongeng_category_id/
// dongeng_sub_category_id. page is 1-indexed; perPage is the page size.
func FindAllFairyTales(db *gorm.DB, search string, page, perPage int, categoryID, subCategoryID string) ([]Dongeng, int64, error) {
	var total int64
	var fairyTales []Dongeng

	applyFilters := func(q *gorm.DB) *gorm.DB {
		q = q.Where("is_deleted = ?", false).Where("hidden = ?", false)
		if search != "" {
			q = q.Where("title ILIKE ?", "%"+search+"%")
		}
		if categoryID != "" {
			q = q.Where("dongeng_category_id = ?", categoryID)
		}
		if subCategoryID != "" {
			q = q.Where("dongeng_sub_category_id = ?", subCategoryID)
		}
		return q
	}

	base := applyFilters(db.Model(&Dongeng{}))
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	findQ := applyFilters(db.Preload("CategoryRef").Preload("SubCategoryRef"))
	offset := (page - 1) * perPage
	if err := findQ.Limit(perPage).Offset(offset).Find(&fairyTales).Error; err != nil {
		return nil, 0, err
	}

	return fairyTales, total, nil
}

// FindFairyTaleByID returns a single visible (not deleted, not hidden) dongeng
// with all its pages and category refs pre-loaded.
func FindFairyTaleByID(db *gorm.DB, id string) (*Dongeng, error) {
	var dongeng Dongeng
	result := db.
		Preload("Pages", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_deleted = ?", false).Order("page_number ASC")
		}).
		Preload("CategoryRef").
		Preload("SubCategoryRef").
		Where("id = ? AND is_deleted = ? AND hidden = ?", id, false, false).
		First(&dongeng)
	if result.Error != nil {
		return nil, result.Error
	}
	return &dongeng, nil
}
