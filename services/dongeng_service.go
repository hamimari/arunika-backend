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

// DongengPageResponse is the DTO for a single page within a dongeng.
type DongengPageResponse struct {
	ID         uuid.UUID `json:"id"`
	DongengId  string    `json:"dongeng_id"`
	PageNumber int       `json:"page_number"`
	ImageUrl   string    `json:"image_url"`
	Text       string    `json:"text"`
	AudioUrl   string    `json:"audio_url"`
}

// DongengResponse is the DTO for a fairy tale.
type DongengResponse struct {
	ID         uuid.UUID             `json:"id"`
	Title      string                `json:"title"`
	AgeStart   float32               `json:"age_start"`
	AgeEnd     float32               `json:"age_end"`
	ImageUrl   string                `json:"image_url"`
	AudioUrl   string                `json:"audio_url"`
	IsFree     bool                  `json:"is_free"`
	CategoryId string                `json:"category_id"`
	Duration   string                `json:"duration"`
	Pages      []DongengPageResponse `json:"pages,omitempty"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
	IsDeleted  bool                  `json:"is_deleted"`
}

// DongengListResult wraps a paginated list response.
type DongengListResult struct {
	Items   []DongengResponse `json:"items"`
	Total   int64             `json:"total"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
}

func NewDongengService(db *gorm.DB) *DongengService {
	return &DongengService{db: db}
}

// GetFairyTales returns a paginated, optionally-searched list of dongengs.
// Pages are not preloaded on the list endpoint (detail endpoint handles that).
func (s *DongengService) GetFairyTales(search string, page, perPage int) (*DongengListResult, error) {
	dongengs, total, err := models.FindAllFairyTales(s.db, search, page, perPage)
	if err != nil {
		return nil, err
	}

	items := make([]DongengResponse, len(dongengs))
	for i, d := range dongengs {
		items[i] = DongengResponse{
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
		}
	}

	return &DongengListResult{Items: items, Total: total, Page: page, PerPage: perPage}, nil
}

// GetFairyTaleByID returns a single dongeng with its pages ordered by page_number.
func (s *DongengService) GetFairyTaleByID(id string) (*DongengResponse, error) {
	dongeng, err := models.FindFairyTaleByID(s.db, id)
	if err != nil {
		return nil, err
	}

	pages := make([]DongengPageResponse, len(dongeng.Pages))
	for i, p := range dongeng.Pages {
		pages[i] = DongengPageResponse{
			ID:         p.ID,
			DongengId:  p.DongengId.String(),
			PageNumber: p.PageNumber,
			ImageUrl:   p.ImageUrl,
			Text:       p.Text,
			AudioUrl:   p.AudioUrl,
		}
	}

	return &DongengResponse{
		ID:         dongeng.ID,
		Title:      dongeng.Title,
		AgeStart:   dongeng.AgeStart,
		AgeEnd:     dongeng.AgeEnd,
		ImageUrl:   dongeng.ImageUrl,
		AudioUrl:   dongeng.AudioUrl,
		IsFree:     dongeng.IsFree,
		CategoryId: dongeng.CategoryId,
		Duration:   fmt.Sprintf("%d min", dongeng.Duration/60),
		Pages:      pages,
		CreatedAt:  dongeng.CreatedAt,
		UpdatedAt:  dongeng.UpdatedAt,
		IsDeleted:  dongeng.IsDeleted,
	}, nil
}
