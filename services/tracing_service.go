package services

import (
	"arunika_backend/models"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log/slog"
)

type TracingService struct {
	db *gorm.DB
}

func NewTracingService(db *gorm.DB) *TracingService {
	return &TracingService{db: db}
}

// GetItems returns tracing items filtered by type.
func (s *TracingService) GetItems(itemType string) ([]models.TracingItem, error) {
	var items []models.TracingItem
	q := s.db.Order("label ASC")
	if itemType != "" {
		q = q.Where("type = ?", itemType)
	}
	if err := q.Find(&items).Error; err != nil {
		slog.Error("TracingService.GetItems", "error", err)
		return nil, err
	}
	return items, nil
}

type SaveTracingProgressRequest struct {
	ItemID  uuid.UUID `json:"item_id" binding:"required"`
	Score   int       `json:"score"   binding:"required,min=0,max=100"`
	Passed  bool      `json:"passed"`
	ChildID uuid.UUID `json:"child_id" binding:"required"`
}

// SaveProgress stores a tracing attempt and awards badges in the same transaction.
func (s *TracingService) SaveProgress(userID uuid.UUID, req SaveTracingProgressRequest) (*models.TracingProgress, error) {
	progress := models.TracingProgress{
		UserID:  userID,
		ChildID: req.ChildID,
		ItemID:  req.ItemID,
		Score:   req.Score,
		Passed:  req.Passed,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&progress).Error; err != nil {
			return fmt.Errorf("create tracing progress: %w", err)
		}
		if req.Passed {
			badgeSvc := NewBadgeService(tx)
			if err := badgeSvc.CheckAndAward(userID, "tracing"); err != nil {
				slog.Warn("badge check failed for tracing", "user_id", userID, "error", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &progress, nil
}

// GuidePathPoints is a helper to deserialise guide_path_json for the API response.
func GuidePathPoints(raw string) ([]map[string]float64, error) {
	var pts []map[string]float64
	if err := json.Unmarshal([]byte(raw), &pts); err != nil {
		return nil, err
	}
	return pts, nil
}
