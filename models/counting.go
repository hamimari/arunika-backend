package models

import (
	"github.com/google/uuid"
	"time"
)

type CountingQuestion struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Level        string    `gorm:"column:level;not null"                           json:"level"`
	QuestionJSON string    `gorm:"column:question_json;type:jsonb;not null"        json:"question_json"`
	Answer       int       `gorm:"column:answer;not null"                          json:"answer"`
	Hidden       bool      `gorm:"column:hidden;default:false"                     json:"hidden"`
	IsDeleted    bool      `gorm:"column:is_deleted;default:false"                 json:"-"`
	UpdatedAt    time.Time `gorm:"column:updated_at"                               json:"updated_at"`
	CreatedAt    time.Time `gorm:"column:created_at"                               json:"created_at"`
}

func (CountingQuestion) TableName() string { return "counting_questions" }

type CountingProgress struct {
	ID         uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID     uuid.UUID `gorm:"column:user_id;not null"                         json:"user_id"`
	ChildID    uuid.UUID `gorm:"column:child_id;not null"                        json:"child_id"`
	QuestionID uuid.UUID `gorm:"column:question_id;not null"                     json:"question_id"`
	IsCorrect  bool      `gorm:"column:is_correct;not null;default:false"        json:"is_correct"`
	CreatedAt  time.Time `gorm:"column:created_at"                               json:"created_at"`
}

func (CountingProgress) TableName() string { return "counting_progress" }
