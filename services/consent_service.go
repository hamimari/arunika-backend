package services

import (
	"arunika_backend/models"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CurrentConsentVersions are the legal document versions users must have
// accepted. Bump a value when the matching document changes materially: every
// user whose latest accepted version differs is then asked to consent again.
var CurrentConsentVersions = map[string]string{
	models.ConsentDocTerms:    "2026-10-01",
	models.ConsentDocPrivacy:  "2026-10-01",
	models.ConsentDocParental: "2026-10-01",
}

// ErrConsentIncomplete means a consent payload is missing a document version.
var ErrConsentIncomplete = errors.New("consent must include terms_version, privacy_version and parental_version")

// ConsentInput is the versions a client says it showed the user.
type ConsentInput struct {
	TermsVersion    string `json:"terms_version"`
	PrivacyVersion  string `json:"privacy_version"`
	ParentalVersion string `json:"parental_version"`
}

// Rows turns the input into consent rows, rejecting a payload that lacks any
// version. The stored version is what the client sent, so the record shows
// what the user was actually shown even if it is already out of date.
func (in ConsentInput) Rows(userID uuid.UUID, ip, userAgent string) ([]models.UserConsent, error) {
	versions := []struct{ doc, version string }{
		{models.ConsentDocTerms, in.TermsVersion},
		{models.ConsentDocPrivacy, in.PrivacyVersion},
		{models.ConsentDocParental, in.ParentalVersion},
	}
	rows := make([]models.UserConsent, 0, len(versions))
	for _, v := range versions {
		version := strings.TrimSpace(v.version)
		if version == "" {
			return nil, ErrConsentIncomplete
		}
		rows = append(rows, models.UserConsent{
			UserID:    userID,
			Document:  v.doc,
			Version:   version,
			IPAddress: ip,
			UserAgent: userAgent,
		})
	}
	return rows, nil
}

type ConsentService struct {
	db *gorm.DB
}

func NewConsentService(db *gorm.DB) *ConsentService {
	return &ConsentService{db: db}
}

// Record appends consent rows for an existing user.
func (s *ConsentService) Record(userID uuid.UUID, in ConsentInput, ip, userAgent string) error {
	rows, err := in.Rows(userID, ip, userAgent)
	if err != nil {
		return err
	}
	return s.db.Create(&rows).Error
}

// IsRequired reports whether the user must consent again: for any document,
// their latest accepted version is missing or differs from the current one.
func (s *ConsentService) IsRequired(userID uuid.UUID) (bool, error) {
	var latest []struct {
		Document string
		Version  string
	}
	err := s.db.Raw(
		`SELECT DISTINCT ON (document) document, version
		   FROM user_consents WHERE user_id = ?
		  ORDER BY document, accepted_at DESC`, userID).Scan(&latest).Error
	if err != nil {
		return false, err
	}
	accepted := make(map[string]string, len(latest))
	for _, l := range latest {
		accepted[l.Document] = l.Version
	}
	for doc, current := range CurrentConsentVersions {
		if accepted[doc] != current {
			return true, nil
		}
	}
	return false, nil
}
