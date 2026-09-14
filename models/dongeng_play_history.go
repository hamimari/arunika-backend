package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DongengPlayHistory tracks per-user watch progress for a dongeng, one row
// per (user_id, dongeng_id) pair.
type DongengPlayHistory struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID          uuid.UUID `gorm:"column:user_id;type:uuid;not null"               json:"user_id"`
	DongengID       uuid.UUID `gorm:"column:dongeng_id;type:uuid;not null"            json:"dongeng_id"`
	ProgressSeconds int       `gorm:"column:progress_seconds;not null;default:0"      json:"progress_seconds"`
	PlayCount       int       `gorm:"column:play_count;not null;default:0"            json:"play_count"`
	StartedAt       time.Time `gorm:"column:started_at"                               json:"started_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"                               json:"updated_at"`
}

func (DongengPlayHistory) TableName() string { return "dongeng_play_history" }

// RecordDongengPlay upserts a play-history row for (userID, dongengID),
// incrementing play_count on repeat plays.
func RecordDongengPlay(db *gorm.DB, userID, dongengID uuid.UUID) error {
	row := DongengPlayHistory{UserID: userID, DongengID: dongengID, PlayCount: 1}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "dongeng_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"play_count": gorm.Expr("play_count + 1"),
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(&row).Error
}

// UpdateDongengProgress upserts the current playback position for
// (userID, dongengID).
func UpdateDongengProgress(db *gorm.DB, userID, dongengID uuid.UUID, progressSeconds int) error {
	row := DongengPlayHistory{UserID: userID, DongengID: dongengID, ProgressSeconds: progressSeconds, PlayCount: 1}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "dongeng_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"progress_seconds": progressSeconds,
			"updated_at":       gorm.Expr("NOW()"),
		}),
	}).Create(&row).Error
}

// DongengHistoryRow is the joined DTO for one play-history entry, with
// total_seconds sourced from the dongeng's own duration rather than
// duplicated into the history table.
type DongengHistoryRow struct {
	DongengID       uuid.UUID `json:"dongeng_id"`
	ProgressSeconds int       `json:"progress_seconds"`
	TotalSeconds    int64     `json:"total_seconds"`
	StartedAt       time.Time `json:"started_at"`
}

// FindDongengHistory returns userID's play history, most recently played first.
func FindDongengHistory(db *gorm.DB, userID uuid.UUID, limit int) ([]DongengHistoryRow, error) {
	var rows []DongengHistoryRow
	err := db.Table("dongeng_play_history h").
		Select("h.dongeng_id, h.progress_seconds, d.duration AS total_seconds, h.started_at").
		Joins("JOIN dongengs d ON d.id = h.dongeng_id AND d.is_deleted = false").
		Where("h.user_id = ?", userID).
		Order("h.updated_at DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}
