package models

import "gorm.io/gorm"

type Dongeng struct {
	BaseModel
	Title      string        `json:"title"       gorm:"column:title"`
	AgeStart   float32       `json:"age_start"   gorm:"column:age_start"`
	AgeEnd     float32       `json:"age_end"     gorm:"column:age_end"`
	ImageUrl   string        `json:"image_url"   gorm:"column:image_url"`
	AudioUrl   string        `json:"audio_url"   gorm:"column:audio_url"`
	IsFree     bool          `json:"is_free"     gorm:"column:is_free"`
	CategoryId string        `json:"category_id" gorm:"column:category_id"`
	Duration   int64         `json:"duration"    gorm:"column:duration"`
	Hidden     bool          `json:"hidden"      gorm:"column:hidden;default:false"`
	Pages      []DongengPage `json:"pages"       gorm:"foreignKey:DongengId"`
}

// FindAllFairyTales returns a paginated, optionally-filtered list of dongengs
// and the total matching count (for the caller to derive hasMore).
// search is case-insensitive title prefix/substring match; empty string = no filter.
// page is 1-indexed; perPage is the page size.
func FindAllFairyTales(db *gorm.DB, search string, page, perPage int) ([]Dongeng, int64, error) {
	var total int64
	var fairyTales []Dongeng

	base := db.Model(&Dongeng{}).Where("is_deleted = ?", false)
	if search != "" {
		base = base.Where("title ILIKE ?", "%"+search+"%")
	}
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	findQ := db.Where("is_deleted = ?", false)
	if search != "" {
		findQ = findQ.Where("title ILIKE ?", "%"+search+"%")
	}
	offset := (page - 1) * perPage
	if err := findQ.Limit(perPage).Offset(offset).Find(&fairyTales).Error; err != nil {
		return nil, 0, err
	}

	return fairyTales, total, nil
}

// FindFairyTaleByID returns a single dongeng with all its pages pre-loaded.
func FindFairyTaleByID(db *gorm.DB, id string) (*Dongeng, error) {
	var dongeng Dongeng
	result := db.
		Preload("Pages", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_deleted = ?", false).Order("page_number ASC")
		}).
		Where("id = ? AND is_deleted = ?", id, false).
		First(&dongeng)
	if result.Error != nil {
		return nil, result.Error
	}
	return &dongeng, nil
}
