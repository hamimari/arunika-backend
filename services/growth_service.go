package services

import (
	"arunika_backend/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type GrowthService struct {
	db *gorm.DB
}

func NewGrowthService(db *gorm.DB) *GrowthService {
	return &GrowthService{db: db}
}

type SaveGrowthRequest struct {
	ChildID    uuid.UUID `json:"child_id"    binding:"required"`
	WeightKg   float64   `json:"weight_kg"   binding:"required,gt=0"`
	HeightCm   float64   `json:"height_cm"   binding:"required,gt=0"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Save stores a new growth record.
func (s *GrowthService) Save(req SaveGrowthRequest) (*models.GrowthRecord, error) {
	if req.RecordedAt.IsZero() {
		req.RecordedAt = time.Now()
	}
	record := models.GrowthRecord{
		ChildID:    req.ChildID,
		WeightKg:   req.WeightKg,
		HeightCm:   req.HeightCm,
		RecordedAt: req.RecordedAt,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

type UpdateGrowthRequest struct {
	WeightKg   float64   `json:"weight_kg"   binding:"required,gt=0"`
	HeightCm   float64   `json:"height_cm"   binding:"required,gt=0"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Update modifies an existing growth record by ID.
func (s *GrowthService) Update(id uuid.UUID, req UpdateGrowthRequest) (*models.GrowthRecord, error) {
	var record models.GrowthRecord
	if err := s.db.First(&record, "id = ?", id).Error; err != nil {
		return nil, err
	}
	record.WeightKg = req.WeightKg
	record.HeightCm = req.HeightCm
	if !req.RecordedAt.IsZero() {
		record.RecordedAt = req.RecordedAt
	}
	if err := s.db.Save(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// GetHistory returns all growth records for a child ordered by recorded_at asc.
func (s *GrowthService) GetHistory(childID uuid.UUID) ([]models.GrowthRecord, error) {
	var records []models.GrowthRecord
	err := s.db.Where("child_id = ?", childID).
		Order("recorded_at ASC").
		Find(&records).Error
	return records, err
}
