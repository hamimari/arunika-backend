package services

import (
	"arunika_backend/models"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type DongengService struct {
	db                 *gorm.DB
	productService     *ProductService
	entitlementService *EntitlementService
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
	ID                   uuid.UUID               `json:"id"`
	Title                string                  `json:"title"`
	AgeStart             float32                 `json:"age_start"`
	AgeEnd               float32                 `json:"age_end"`
	ImageUrl             string                  `json:"image_url"`
	AudioUrl             string                  `json:"audio_url"`
	IsFree               bool                    `json:"is_free"`
	IsUnlocked           bool                    `json:"is_unlocked"`
	ProductID            *uuid.UUID              `json:"product_id,omitempty"`
	PriceIdr             *int64                  `json:"price_idr,omitempty"`
	CategoryId           string                  `json:"category_id"`
	DongengCategoryID    *uuid.UUID              `json:"dongeng_category_id,omitempty"`
	DongengSubCategoryID *uuid.UUID              `json:"dongeng_sub_category_id,omitempty"`
	CategoryRef          *models.DongengCategory `json:"category_ref,omitempty"`
	SubCategoryRef       *models.DongengCategory `json:"sub_category_ref,omitempty"`
	Duration             string                  `json:"duration"`
	Pages                []DongengPageResponse   `json:"pages,omitempty"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
	IsDeleted            bool                    `json:"is_deleted"`
}

// DongengListResult wraps a paginated list response.
type DongengListResult struct {
	Items   []DongengResponse `json:"items"`
	Total   int64             `json:"total"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
}

func NewDongengService(db *gorm.DB, productService *ProductService, entitlementService *EntitlementService) *DongengService {
	return &DongengService{db: db, productService: productService, entitlementService: entitlementService}
}

// GetCategories returns all top-level dongeng categories with their
// sub-categories preloaded.
func (s *DongengService) GetCategories() ([]models.DongengCategory, error) {
	return models.FindTopLevelDongengCategories(s.db)
}

// computeUnlocked mirrors ArService.applyUnlocked: free content (no linked
// product) is always unlocked; otherwise it depends on the requesting user's
// entitlements/subscription. Also returns the linked product (nil for free
// content) so callers can surface its id/price to the client.
func (s *DongengService) computeUnlocked(dongengID uuid.UUID, isFree bool, userID *uuid.UUID) (bool, *models.Product, error) {
	if isFree {
		return true, nil, nil
	}
	product, err := s.productService.ResolveByDongengID(dongengID)
	if err != nil {
		return false, nil, err
	}
	if product == nil {
		return true, nil, nil
	}

	var unlocked bool
	if userID != nil {
		unlocked, err = s.entitlementService.HasAccess(*userID, product.ID)
		if err != nil {
			return false, nil, err
		}
	}

	// Inactive products are withdrawn from sale — hide the purchase option
	// from anyone who doesn't already own it. Existing owners (unlocked via
	// entitlement/subscription) keep full access, unaffected by is_active.
	if !product.IsActive && !unlocked {
		return false, nil, nil
	}

	return unlocked, product, nil
}

// GetFairyTales returns a paginated, optionally-searched/category-filtered
// list of dongengs. Pages are not preloaded on the list endpoint (detail
// endpoint handles that). categoryID/subCategoryID, when non-empty, filter
// to that dongeng_category_id/dongeng_sub_category_id.
func (s *DongengService) GetFairyTales(search string, page, perPage int, userID *uuid.UUID, categoryID, subCategoryID string) (*DongengListResult, error) {
	dongengs, total, err := models.FindAllFairyTales(s.db, search, page, perPage, categoryID, subCategoryID)
	if err != nil {
		return nil, err
	}

	items := make([]DongengResponse, len(dongengs))
	for i, d := range dongengs {
		unlocked, product, err := s.computeUnlocked(d.ID, d.IsFree, userID)
		if err != nil {
			return nil, err
		}
		items[i] = DongengResponse{
			ID:                   d.ID,
			Title:                d.Title,
			AgeStart:             d.AgeStart,
			AgeEnd:               d.AgeEnd,
			ImageUrl:             d.ImageUrl,
			AudioUrl:             d.AudioUrl,
			IsFree:               d.IsFree,
			IsUnlocked:           unlocked,
			CategoryId:           uuidString(d.CategoryId),
			DongengCategoryID:    d.DongengCategoryID,
			DongengSubCategoryID: d.DongengSubCategoryID,
			CategoryRef:          d.CategoryRef,
			SubCategoryRef:       d.SubCategoryRef,
			Duration:             fmt.Sprintf("%d min", d.Duration/60),
			CreatedAt:            d.CreatedAt,
			UpdatedAt:            d.UpdatedAt,
			IsDeleted:            d.IsDeleted,
		}
		if product != nil {
			items[i].ProductID = &product.ID
			items[i].PriceIdr = &product.PriceIdr
		}
	}

	return &DongengListResult{Items: items, Total: total, Page: page, PerPage: perPage}, nil
}

// RecordPlay records that userID started playing dongengID.
func (s *DongengService) RecordPlay(userID, dongengID uuid.UUID) error {
	return models.RecordDongengPlay(s.db, userID, dongengID)
}

// UpdateProgress persists userID's current playback position for dongengID.
func (s *DongengService) UpdateProgress(userID, dongengID uuid.UUID, progressSeconds int) error {
	return models.UpdateDongengProgress(s.db, userID, dongengID, progressSeconds)
}

// DongengHistoryItem is the DTO for a single watch-history entry.
type DongengHistoryItem struct {
	DongengID       uuid.UUID `json:"dongeng_id"`
	ProgressSeconds int       `json:"progress_seconds"`
	TotalSeconds    int64     `json:"total_seconds"`
	StartedAt       time.Time `json:"started_at"`
}

// GetHistory returns userID's most recently played dongengs.
func (s *DongengService) GetHistory(userID uuid.UUID) ([]DongengHistoryItem, error) {
	rows, err := models.FindDongengHistory(s.db, userID, 20)
	if err != nil {
		return nil, err
	}
	items := make([]DongengHistoryItem, len(rows))
	for i, r := range rows {
		items[i] = DongengHistoryItem{
			DongengID:       r.DongengID,
			ProgressSeconds: r.ProgressSeconds,
			TotalSeconds:    r.TotalSeconds,
			StartedAt:       r.StartedAt,
		}
	}
	return items, nil
}

// GetFairyTaleByID returns a single dongeng with its pages ordered by page_number.
func (s *DongengService) GetFairyTaleByID(id string, userID *uuid.UUID) (*DongengResponse, error) {
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

	unlocked, product, err := s.computeUnlocked(dongeng.ID, dongeng.IsFree, userID)
	if err != nil {
		return nil, err
	}

	resp := &DongengResponse{
		ID:                   dongeng.ID,
		Title:                dongeng.Title,
		AgeStart:             dongeng.AgeStart,
		AgeEnd:               dongeng.AgeEnd,
		ImageUrl:             dongeng.ImageUrl,
		AudioUrl:             dongeng.AudioUrl,
		IsFree:               dongeng.IsFree,
		IsUnlocked:           unlocked,
		CategoryId:           uuidString(dongeng.CategoryId),
		DongengCategoryID:    dongeng.DongengCategoryID,
		DongengSubCategoryID: dongeng.DongengSubCategoryID,
		CategoryRef:          dongeng.CategoryRef,
		SubCategoryRef:       dongeng.SubCategoryRef,
		Duration:             fmt.Sprintf("%d min", dongeng.Duration/60),
		Pages:                pages,
		CreatedAt:            dongeng.CreatedAt,
		UpdatedAt:            dongeng.UpdatedAt,
		IsDeleted:            dongeng.IsDeleted,
	}
	if product != nil {
		resp.ProductID = &product.ID
		resp.PriceIdr = &product.PriceIdr
	}
	return resp, nil
}

// uuidString renders an optional UUID for responses that expose it as a plain
// string, with "" for unset.
func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
