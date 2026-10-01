package services

import (
	"arunika_backend/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log/slog"
	"time"
)

type BadgeService struct {
	db *gorm.DB
}

func NewBadgeService(db *gorm.DB) *BadgeService {
	return &BadgeService{db: db}
}

// CheckAndAward evaluates badge thresholds for the given feature and inserts
// any newly earned badges. Uses ON CONFLICT DO NOTHING for idempotency.
// Designed to be called inside an existing transaction.
func (s *BadgeService) CheckAndAward(userID uuid.UUID, feature string) error {
	// Count passed items for this feature.
	var passedCount int64
	switch feature {
	case "tracing":
		s.db.Model(&models.TracingProgress{}).
			Where("user_id = ? AND passed = true", userID).
			Count(&passedCount)
	case "counting":
		s.db.Model(&models.CountingProgress{}).
			Where("user_id = ? AND is_correct = true", userID).
			Count(&passedCount)
	}

	// Load badge definitions for this feature.
	var badges []models.Badge
	if err := s.db.Where("feature = ?", feature).Find(&badges).Error; err != nil {
		return err
	}

	now := time.Now()
	for _, badge := range badges {
		if int64(badge.Threshold) <= passedCount {
			row := models.UserBadge{
				UserID:   userID,
				BadgeID:  badge.ID,
				EarnedAt: now,
			}
			if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				slog.Error("BadgeService.CheckAndAward insert", "badge", badge.Level, "error", err)
			}
		}
	}

	// Check All-Rounder: user must have ≥1 beginner badge for every non-global feature.
	s.checkAllRounder(userID, now)
	return nil
}

func (s *BadgeService) checkAllRounder(userID uuid.UUID, now time.Time) {
	features := []string{"tracing", "counting"}
	for _, f := range features {
		var count int64
		s.db.Model(&models.UserBadge{}).
			Joins("JOIN badges ON badges.id = user_badges.badge_id").
			Where("user_badges.user_id = ? AND badges.feature = ? AND badges.level = 'beginner'", userID, f).
			Count(&count)
		if count == 0 {
			return // not all features covered
		}
	}
	// Award global all_rounder badge.
	var globalBadge models.Badge
	if err := s.db.Where("feature = 'global' AND level = 'all_rounder'").First(&globalBadge).Error; err != nil {
		return
	}
	row := models.UserBadge{UserID: userID, BadgeID: globalBadge.ID, EarnedAt: now}
	s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
}

// GetUserBadges returns all badges with earned status and progress for a user.
type BadgeWithProgress struct {
	models.Badge
	Earned   bool  `json:"earned"`
	Progress int64 `json:"progress"`
}

func (s *BadgeService) GetUserBadges(userID uuid.UUID) ([]BadgeWithProgress, error) {
	var badges []models.Badge
	if err := s.db.Find(&badges).Error; err != nil {
		return nil, err
	}

	var earnedBadgeIDs []uuid.UUID
	s.db.Model(&models.UserBadge{}).
		Where("user_id = ?", userID).
		Pluck("badge_id", &earnedBadgeIDs)
	earnedSet := map[uuid.UUID]bool{}
	for _, id := range earnedBadgeIDs {
		earnedSet[id] = true
	}

	// Progress counts per feature.
	var tracingCount, countingCount int64
	s.db.Model(&models.TracingProgress{}).Where("user_id = ? AND passed = true", userID).Count(&tracingCount)
	s.db.Model(&models.CountingProgress{}).Where("user_id = ? AND is_correct = true", userID).Count(&countingCount)

	result := make([]BadgeWithProgress, 0, len(badges))
	for _, b := range badges {
		var progress int64
		switch b.Feature {
		case "tracing":
			progress = tracingCount
		case "counting":
			progress = countingCount
		case "global":
			progress = 0
		}
		result = append(result, BadgeWithProgress{
			Badge:    b,
			Earned:   earnedSet[b.ID],
			Progress: progress,
		})
	}
	return result, nil
}
