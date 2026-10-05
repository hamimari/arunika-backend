package models

import (
	"time"

	"github.com/google/uuid"
)

// Belajar Angka statuses. Objects, numbers and levels share Huruf's
// draft / published / hidden lifecycle.
const (
	AngkaStatusDraft     = "draft"
	AngkaStatusPublished = "published"
	AngkaStatusHidden    = "hidden"

	AngkaSessionInProgress = "in_progress"
	AngkaSessionCompleted  = "completed"
	AngkaSessionAbandoned  = "abandoned"
)

// AngkaObject is a library picture questions count ("apel"). Content is
// the published copy (nil until first published).
type AngkaObject struct {
	ID        uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Status    string     `gorm:"not null;default:draft"                          json:"status"`
	Draft     JSONB      `gorm:"type:jsonb;not null"                             json:"draft"`
	Content   JSONB      `gorm:"type:jsonb"                                      json:"content"`
	DraftRev  int        `gorm:"not null;default:1"                              json:"draft_rev"`
	UpdatedBy *uuid.UUID `gorm:"type:uuid"                                       json:"-"`
	CreatedAt time.Time  `gorm:"autoCreateTime"                                  json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"                                  json:"updated_at"`
}

func (AngkaObject) TableName() string { return "angka_objects" }

// AngkaNumber is one Kenal Angka card, 1–20.
type AngkaNumber struct {
	Value     int        `gorm:"primaryKey;autoIncrement:false" json:"value"`
	IsFree    bool       `gorm:"not null;default:false"         json:"is_free"`
	Status    string     `gorm:"not null;default:draft"         json:"status"`
	Draft     JSONB      `gorm:"type:jsonb;not null"            json:"draft"`
	Content   JSONB      `gorm:"type:jsonb"                     json:"content"`
	DraftRev  int        `gorm:"not null;default:1"             json:"draft_rev"`
	UpdatedBy *uuid.UUID `gorm:"type:uuid"                      json:"-"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime"                 json:"updated_at"`
}

func (AngkaNumber) TableName() string { return "angka_numbers" }

// AngkaLevel is a Hitung Benda level. The app only ever reads the
// AngkaLevelVersion that PublishedVersionID points at.
type AngkaLevel struct {
	ID                 uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	SortOrder          int        `gorm:"not null"                                        json:"sort_order"`
	IsFree             bool       `gorm:"not null;default:false"                          json:"is_free"`
	Status             string     `gorm:"not null;default:draft"                          json:"status"`
	Draft              JSONB      `gorm:"type:jsonb;not null"                             json:"draft"`
	DraftRev           int        `gorm:"not null;default:1"                              json:"draft_rev"`
	PublishedVersionID *uuid.UUID `gorm:"type:uuid"                                       json:"published_version_id"`
	UpdatedBy          *uuid.UUID `gorm:"type:uuid"                                       json:"-"`
	UpdatedAt          time.Time  `gorm:"autoUpdateTime"                                  json:"updated_at"`
}

func (AngkaLevel) TableName() string { return "angka_levels" }

// AngkaLevelVersion is an immutable published snapshot of a level.
type AngkaLevelVersion struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	LevelID     uuid.UUID `gorm:"type:uuid;not null"                              json:"level_id"`
	Version     int       `gorm:"not null"                                        json:"version"`
	Content     JSONB     `gorm:"type:jsonb;not null"                             json:"content"`
	PublishedBy uuid.UUID `gorm:"type:uuid;not null"                              json:"-"`
	PublishedAt time.Time `gorm:"autoCreateTime"                                  json:"published_at"`
}

func (AngkaLevelVersion) TableName() string { return "angka_level_versions" }

// AngkaSession is one play-through of a level: the seed and object snapshot
// regenerate its questions; Answers holds every try.
type AngkaSession struct {
	ID             uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ChildID        uuid.UUID `gorm:"type:uuid;not null"`
	LevelID        uuid.UUID `gorm:"type:uuid;not null"`
	LevelVersionID uuid.UUID `gorm:"type:uuid;not null"`
	Seed           int64     `gorm:"not null"`
	ObjectIDs      JSONB     `gorm:"column:object_ids;type:jsonb;not null"`
	Answers        JSONB     `gorm:"type:jsonb;not null"`
	Status         string    `gorm:"not null;default:in_progress"`
	Stars          *int      `gorm:"type:smallint"`
	FirstCorrect   *int      `gorm:"type:smallint"`
	Unlocked       JSONB     `gorm:"type:jsonb"`
	StartedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
	CompletedAt    *time.Time
}

func (AngkaSession) TableName() string { return "angka_sessions" }

// AngkaLevelProgress is one child's best result on one level.
type AngkaLevelProgress struct {
	ChildID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	LevelID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	BestStars        int        `gorm:"type:smallint;not null;default:0"`
	Completed        bool       `gorm:"not null;default:false"`
	CurrentSessionID *uuid.UUID `gorm:"type:uuid"`
	UpdatedAt        time.Time  `gorm:"not null"`
}

func (AngkaLevelProgress) TableName() string { return "angka_level_progress" }

// AngkaObjectContent is the shape of AngkaObject.Draft and .Content. Each
// media slot holds an uploaded asset id or an external https URL; the asset
// id wins.
type AngkaObjectContent struct {
	Name             string     `json:"name"`
	QuestionText     string     `json:"question_text"`
	ImageAssetID     *uuid.UUID `json:"image_asset_id"`
	ImageURL         string     `json:"image_url,omitempty"`
	QuestionAudioID  *uuid.UUID `json:"question_audio_id"`
	QuestionAudioURL string     `json:"question_audio_url,omitempty"`
}

// AngkaNumberContent is the shape of AngkaNumber.Draft and .Content.
type AngkaNumberContent struct {
	Name         string     `json:"name"`
	AudioAssetID *uuid.UUID `json:"audio_asset_id"`
	AudioURL     string     `json:"audio_url,omitempty"`
	ObjectID     *uuid.UUID `json:"object_id"`
}

// AngkaLevelContent is the shape of AngkaLevel.Draft and version content.
type AngkaLevelContent struct {
	Name           string             `json:"name"`
	PrerequisiteID *uuid.UUID         `json:"prerequisite_id"`
	Range          AngkaRange         `json:"range"`
	QuestionCount  int                `json:"question_count"`
	ObjectIDs      []uuid.UUID        `json:"object_ids"`
	Layout         string             `json:"layout"`
	Stars          AngkaStars         `json:"stars"`
	Feedback       AngkaLevelFeedback `json:"feedback"`
}

type AngkaRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// AngkaStars are the first-try-correct thresholds for 3 and 2 stars.
type AngkaStars struct {
	Three int `json:"three"`
	Two   int `json:"two"`
}

type AngkaLevelFeedback struct {
	Success string `json:"success"`
	Retry   string `json:"retry"`
	Hint    string `json:"hint"`
}

// AngkaAnswer is every try on one question (1-based Q).
type AngkaAnswer struct {
	Q     int   `json:"q"`
	Tries []int `json:"tries"`
}
