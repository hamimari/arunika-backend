package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	LetterStatusDraft     = "draft"
	LetterStatusPublished = "published"
	LetterStatusHidden    = "hidden"
)

// Letter is one Belajar Huruf letter. Admins edit Draft; the app only ever
// reads the LetterVersion that PublishedVersionID points at.
type Letter struct {
	ID                 uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Upper              string     `gorm:"not null"                                        json:"upper"`
	SortOrder          int        `gorm:"not null"                                        json:"sort_order"`
	IsFree             bool       `gorm:"not null;default:false"                          json:"is_free"`
	Status             string     `gorm:"not null;default:draft"                          json:"status"`
	Draft              JSONB      `gorm:"type:jsonb;not null"                             json:"draft"`
	DraftRev           int        `gorm:"not null;default:1"                              json:"draft_rev"`
	PublishedVersionID *uuid.UUID `gorm:"type:uuid"                                       json:"published_version_id"`
	UpdatedBy          *uuid.UUID `gorm:"type:uuid"                                       json:"-"`
	UpdatedAt          time.Time  `gorm:"autoUpdateTime"                                  json:"updated_at"`
}

func (Letter) TableName() string { return "letters" }

// LetterVersion is an immutable published snapshot of a letter's content.
type LetterVersion struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	LetterID    uuid.UUID `gorm:"type:uuid;not null"                              json:"letter_id"`
	Version     int       `gorm:"not null"                                        json:"version"`
	Content     JSONB     `gorm:"type:jsonb;not null"                             json:"content"`
	PublishedBy uuid.UUID `gorm:"type:uuid;not null"                              json:"-"`
	PublishedAt time.Time `gorm:"autoCreateTime"                                  json:"published_at"`
}

func (LetterVersion) TableName() string { return "letter_versions" }

// LetterProgress is one child's progress on one letter.
type LetterProgress struct {
	ChildID      uuid.UUID `gorm:"type:uuid;primaryKey"      json:"-"`
	LetterID     uuid.UUID `gorm:"type:uuid;primaryKey"      json:"letter_id"`
	KenaliDone   bool      `gorm:"not null;default:false"    json:"kenali_done"`
	DengarDone   bool      `gorm:"not null;default:false"    json:"dengar_done"`
	TebalkanDone bool      `gorm:"not null;default:false"    json:"tebalkan_done"`
	BestScore    *float64  `gorm:"type:numeric(3,2)"         json:"best_score"`
	Attempts     int       `gorm:"not null;default:0"        json:"attempts"`
	UpdatedAt    time.Time `gorm:"not null"                  json:"updated_at"`
}

func (LetterProgress) TableName() string { return "letter_progress" }

// LetterContent is the shape of Letter.Draft and LetterVersion.Content.
type LetterContent struct {
	Upper    string         `json:"upper"`
	Lower    string         `json:"lower"`
	Kenali   LetterKenali   `json:"kenali"`
	Dengar   LetterDengar   `json:"dengar"`
	Tebalkan LetterTebalkan `json:"tebalkan"`
	Feedback LetterFeedback `json:"feedback"`
}

// Each media slot holds either an uploaded asset id or an external https URL
// (a file the content team uploaded to R2 themselves); the asset id wins.
type LetterKenali struct {
	Word         string     `json:"word"`
	Highlight    []int      `json:"highlight"`
	ImageAssetID *uuid.UUID `json:"image_asset_id"`
	ImageURL     string     `json:"image_url,omitempty"`
}

type LetterDengar struct {
	LetterAudioID  *uuid.UUID `json:"letter_audio_id"`
	WordAudioID    *uuid.UUID `json:"word_audio_id"`
	LetterAudioURL string     `json:"letter_audio_url,omitempty"`
	WordAudioURL   string     `json:"word_audio_url,omitempty"`
}

type LetterTebalkan struct {
	Grid          int            `json:"grid"`
	LowerRequired bool           `json:"lower_required"`
	Upper         []LetterStroke `json:"upper"`
	Lower         []LetterStroke `json:"lower"`
}

type LetterStroke struct {
	Order int    `json:"order"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

type LetterFeedback struct {
	Success string `json:"success"`
	Retry   string `json:"retry"`
	Hint    string `json:"hint"`
}
