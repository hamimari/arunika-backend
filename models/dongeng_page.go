package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DongengPage represents a single page in a dongeng (fairy tale).
type DongengPage struct {
	BaseModel
	DongengId  uuid.UUID `json:"dongeng_id"  gorm:"column:dongeng_id;not null;type:uuid"`
	PageNumber int       `json:"page_number" gorm:"column:page_number;not null"`
	ImageUrl   string    `json:"image_url"   gorm:"column:image_url;not null"`
	Text       string    `json:"text"        gorm:"column:text;not null"`
	AudioUrl   string    `json:"audio_url"   gorm:"column:audio_url;not null;default:''"`
}

func (DongengPage) TableName() string {
	return "dongeng_pages"
}

// FindPagesByDongengId returns all active pages for a dongeng ordered by page number.
func FindPagesByDongengId(db *gorm.DB, dongengId string) ([]DongengPage, error) {
	var pages []DongengPage
	result := db.
		Where("dongeng_id = ? AND is_deleted = ?", dongengId, false).
		Order("page_number ASC").
		Find(&pages)
	return pages, result.Error
}
