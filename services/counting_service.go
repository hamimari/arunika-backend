package services

import (
	"arunika_backend/models"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log/slog"
)

type CountingService struct {
	db *gorm.DB
}

func NewCountingService(db *gorm.DB) *CountingService {
	return &CountingService{db: db}
}

// GetQuestions returns counting questions filtered by level.
func (s *CountingService) GetQuestions(level string) ([]models.CountingQuestion, error) {
	var questions []models.CountingQuestion
	q := s.db
	if level != "" {
		q = q.Where("level = ?", level)
	}
	if err := q.Find(&questions).Error; err != nil {
		slog.Error("CountingService.GetQuestions", "error", err)
		return nil, err
	}
	return questions, nil
}

type SaveCountingProgressRequest struct {
	QuestionID uuid.UUID `json:"question_id" binding:"required"`
	IsCorrect  bool      `json:"is_correct"`
	ChildID    uuid.UUID `json:"child_id"    binding:"required"`
}

// SaveProgress stores a counting answer and awards badges in the same transaction.
func (s *CountingService) SaveProgress(userID uuid.UUID, req SaveCountingProgressRequest) (*models.CountingProgress, error) {
	progress := models.CountingProgress{
		UserID:     userID,
		ChildID:    req.ChildID,
		QuestionID: req.QuestionID,
		IsCorrect:  req.IsCorrect,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&progress).Error; err != nil {
			return fmt.Errorf("create counting progress: %w", err)
		}
		if req.IsCorrect {
			badgeSvc := NewBadgeService(tx)
			if err := badgeSvc.CheckAndAward(userID, "counting"); err != nil {
				slog.Warn("badge check failed for counting", "user_id", userID, "error", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &progress, nil
}
