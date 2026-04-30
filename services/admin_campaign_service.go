package services

import (
	"arunika_backend/models"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log/slog"
	"math"
	"time"
)

type AdminCampaignService struct {
	db              *gorm.DB
	notificationSvc *NotificationService
}

func NewAdminCampaignService(db *gorm.DB, ns *NotificationService) *AdminCampaignService {
	return &AdminCampaignService{db: db, notificationSvc: ns}
}

type CampaignRequest struct {
	Title   string `json:"title"   binding:"required"`
	Body    string `json:"body"    binding:"required"`
	Channel string `json:"channel" binding:"required"` // push | email | both
	Segment string `json:"segment" binding:"required"` // all | subscribers
}

type CampaignResult struct {
	Sent   int `json:"sent"`
	Failed int `json:"failed"`
}

// Dispatch sends a campaign notification to the selected user segment.
// It returns immediately with a result; dispatch is synchronous for simplicity.
func (s *AdminCampaignService) Dispatch(req CampaignRequest) (*CampaignResult, error) {
	users, err := s.resolveSegment(req.Segment)
	if err != nil {
		return nil, err
	}

	result := &CampaignResult{}
	const chunkSize = 500

	for i := 0; i < len(users); i += chunkSize {
		end := int(math.Min(float64(i+chunkSize), float64(len(users))))
		chunk := users[i:end]
		for _, uid := range chunk {
			if req.Channel == "push" || req.Channel == "both" {
				if err := s.notificationSvc.Send(uid, req.Title, req.Body, "campaign"); err != nil {
					slog.Warn("campaign: FCM send failed", "user_id", uid, "error", err)
					result.Failed++
					continue
				}
			}
			if req.Channel == "email" || req.Channel == "both" {
				// Fetch user email for sending
				var parent models.Parent
				if err := s.db.Where("id = ? AND is_deleted = false", uid).Select("email_address").First(&parent).Error; err == nil {
					if err := sendCampaignEmail(parent.EmailAddress, req.Title, req.Body); err != nil {
						slog.Warn("campaign: email send failed", "user_id", uid, "error", err)
						result.Failed++
						continue
					}
				}
			}
			result.Sent++
		}
		// Small pause between chunks to respect FCM rate limits.
		if end < len(users) {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return result, nil
}

func (s *AdminCampaignService) resolveSegment(segment string) ([]uuid.UUID, error) {
	var uids []uuid.UUID
	switch segment {
	case "subscribers":
		var subs []models.UserSubscription
		if err := s.db.Where("status = 'premium'").Find(&subs).Error; err != nil {
			return nil, err
		}
		for _, sub := range subs {
			uids = append(uids, sub.UserID)
		}
	default: // "all"
		var parents []models.Parent
		if err := s.db.Where("is_deleted = false").Find(&parents).Error; err != nil {
			return nil, err
		}
		for _, p := range parents {
			uids = append(uids, p.ID)
		}
	}
	return uids, nil
}

// sendCampaignEmail sends a simple HTML email campaign message.
func sendCampaignEmail(to, subject, body string) error {
	html := fmt.Sprintf(`<html><body><h2>%s</h2><p>%s</p></body></html>`, subject, body)
	return SendGenericEmail(to, subject, html)
}
