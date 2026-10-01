package models

import (
	"time"

	"github.com/google/uuid"
)

// Consent documents a parent accepts. PARENTAL is the parent/guardian
// declaration that lets us process the child's data (UU PDP Art. 25).
const (
	ConsentDocTerms    = "TERMS"
	ConsentDocPrivacy  = "PRIVACY"
	ConsentDocParental = "PARENTAL"
)

// UserConsent is one acceptance of one document version. Rows are only ever
// appended, so the history of what was accepted and when is kept as evidence.
type UserConsent struct {
	ID         uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	UserID     uuid.UUID `gorm:"column:user_id;not null"                         json:"user_id"`
	Document   string    `gorm:"column:document;not null"                        json:"document"`
	Version    string    `gorm:"column:version;not null"                         json:"version"`
	AcceptedAt time.Time `gorm:"column:accepted_at;not null;autoCreateTime"      json:"accepted_at"`
	IPAddress  string    `gorm:"column:ip_address"                               json:"-"`
	UserAgent  string    `gorm:"column:user_agent"                               json:"-"`
}

func (UserConsent) TableName() string { return "user_consents" }
