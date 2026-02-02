package services

import (
	"arunika_backend/models"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type DongengService struct {
	db *gorm.DB
}

type DongengResponse struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	AgeStart   float32   `json:"age_start"`
	AgeEnd     float32   `json:"age_end"`
	ImageUrl   string    `json:"image_url"`
	AudioUrl   string    `json:"audio_url"`
	IsFree     bool      `json:"is_free"`
	CategoryId string    `json:"category_id"`
	Duration   string    `json:"duration"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	IsDeleted  bool      `json:"is_deleted"`
}

func NewDongengService(db *gorm.DB) *DongengService {
	return &DongengService{db: db}
}

func (s *DongengService) GetFairyTales() ([]DongengResponse, error) {
	dongengs, err := models.FindAllFairyTales(s.db)
	if err != nil {
		return nil, err
	}
	var dongengResponse []DongengResponse
	for _, d := range dongengs {
		dongengResponse = append(dongengResponse, DongengResponse{
			ID:         d.ID,
			Title:      d.Title,
			AgeStart:   d.AgeStart,
			AgeEnd:     d.AgeEnd,
			ImageUrl:   d.ImageUrl,
			AudioUrl:   d.AudioUrl,
			IsFree:     d.IsFree,
			CategoryId: d.CategoryId,
			Duration:   fmt.Sprintf("%d min", d.Duration/60),
			CreatedAt:  d.CreatedAt,
			UpdatedAt:  d.UpdatedAt,
			IsDeleted:  d.IsDeleted,
		})
	}
	return dongengResponse, nil
}
