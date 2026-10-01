package services

import (
	"arunika_backend/models"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// ErrChildNotFound covers both "no such child/record" and "it belongs to
// someone else". Callers must answer both identically, or the endpoint becomes
// an oracle for which child ids exist.
var ErrChildNotFound = errors.New("child not found")

type GrowthService struct {
	db *gorm.DB
}

func NewGrowthService(db *gorm.DB) *GrowthService {
	return &GrowthService{db: db}
}

// ownsChild reports whether childID is one of userID's children.
func (s *GrowthService) ownsChild(userID, childID uuid.UUID) error {
	var count int64
	if err := s.db.Table("children").
		Where("id = ? AND parent_id = ?", childID, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrChildNotFound
	}
	return nil
}

type SaveGrowthRequest struct {
	ChildID    uuid.UUID `json:"child_id"    binding:"required"`
	WeightKg   float64   `json:"weight_kg"   binding:"required,gt=0"`
	HeightCm   float64   `json:"height_cm"   binding:"required,gt=0"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Save stores a new growth record for one of userID's children.
func (s *GrowthService) Save(userID uuid.UUID, req SaveGrowthRequest) (*models.GrowthRecord, error) {
	if err := s.ownsChild(userID, req.ChildID); err != nil {
		return nil, err
	}
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

// Update modifies an existing growth record by ID, if it belongs to one of
// userID's children.
func (s *GrowthService) Update(userID, id uuid.UUID, req UpdateGrowthRequest) (*models.GrowthRecord, error) {
	var record models.GrowthRecord
	if err := s.db.First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChildNotFound
		}
		return nil, err
	}
	if err := s.ownsChild(userID, record.ChildID); err != nil {
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

// GetHistory returns all growth records for one of userID's children ordered
// by recorded_at asc.
func (s *GrowthService) GetHistory(userID, childID uuid.UUID) ([]models.GrowthRecord, error) {
	if err := s.ownsChild(userID, childID); err != nil {
		return nil, err
	}
	var records []models.GrowthRecord
	err := s.db.Where("child_id = ?", childID).
		Order("recorded_at ASC").
		Find(&records).Error
	return records, err
}
