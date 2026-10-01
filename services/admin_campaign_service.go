package services

import (
	"arunika_backend/models"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AdminCampaignService struct {
	db              *gorm.DB
	notificationSvc *NotificationService
	// runAsync launches background delivery; tests swap it for a
	// synchronous call so sqlmock expectations stay deterministic.
	runAsync func(func())
}

func NewAdminCampaignService(db *gorm.DB, ns *NotificationService) *AdminCampaignService {
	return &AdminCampaignService{db: db, notificationSvc: ns, runAsync: func(f func()) { go f() }}
}

type CampaignRequest struct {
	Title    string `json:"title"     binding:"required,max=200"`
	Body     string `json:"body"      binding:"required"`
	Channel  string `json:"channel"   binding:"required,oneof=push email both"`
	Segment  string `json:"segment"   binding:"required,oneof=all_devices all subscribers"`
	ImageURL string `json:"image_url"`
	LinkType string `json:"link_type"` // none (default) | ar_card | dongeng
	LinkID   string `json:"link_id"`   // AR card / dongeng id, required unless link_type is none
}

// CampaignValidationError marks a request the handler should reject with 400.
type CampaignValidationError struct{ msg string }

func (e *CampaignValidationError) Error() string { return e.msg }

// ErrPushUnavailable wraps a push campaign rejected because FCM isn't set up;
// the handler reports it as 503 with the underlying reason.
var ErrPushUnavailable = errors.New("push notifications are not available")

func invalidCampaign(format string, args ...interface{}) error {
	return &CampaignValidationError{msg: fmt.Sprintf(format, args...)}
}

const (
	campaignChunkSize   = 500
	campaignConcurrency = 8
)

// Dispatch validates the request, records the campaign, and starts delivery
// in the background. The returned campaign is in SENDING state; poll List to
// follow its progress.
func (s *AdminCampaignService) Dispatch(req CampaignRequest, adminID *uuid.UUID) (*models.Campaign, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	req.ImageURL = strings.TrimSpace(req.ImageURL)
	req.LinkID = strings.TrimSpace(req.LinkID)
	if req.LinkType == "" {
		req.LinkType = models.CampaignLinkNone
	}
	if err := s.validate(&req); err != nil {
		return nil, err
	}
	// Fail before recording anything if pushes can't go out at all, instead
	// of creating a campaign that is guaranteed to end FAILED.
	if req.Channel == "push" || req.Channel == "both" {
		if err := s.notificationSvc.CheckPushConfigured(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPushUnavailable, err)
		}
	}

	campaign := models.Campaign{
		Title:     req.Title,
		Body:      req.Body,
		ImageURL:  req.ImageURL,
		Channel:   req.Channel,
		Segment:   req.Segment,
		LinkType:  req.LinkType,
		LinkID:    req.LinkID,
		Status:    models.CampaignStatusSending,
		CreatedBy: adminID,
	}
	if err := s.db.Create(&campaign).Error; err != nil {
		return nil, err
	}

	snapshot := campaign
	s.runAsync(func() { s.deliver(snapshot) })
	return &campaign, nil
}

func (s *AdminCampaignService) validate(req *CampaignRequest) error {
	if req.Title == "" || req.Body == "" {
		return invalidCampaign("title and body are required")
	}
	if req.Segment == models.CampaignSegmentAllDevices && req.Channel != "push" {
		return invalidCampaign("the all_devices segment only supports the push channel")
	}
	if req.ImageURL != "" && !strings.HasPrefix(req.ImageURL, "https://") {
		return invalidCampaign("image_url must be an https URL")
	}

	switch req.LinkType {
	case models.CampaignLinkNone:
		req.LinkID = ""
	case models.CampaignLinkArCard:
		if req.LinkID == "" {
			return invalidCampaign("link_id is required for link_type ar_card")
		}
		var count int64
		if err := s.db.Model(&models.ArCards{}).Where("id = ?", req.LinkID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return invalidCampaign("AR card %q not found", req.LinkID)
		}
	case models.CampaignLinkDongeng:
		id, err := uuid.Parse(req.LinkID)
		if err != nil {
			return invalidCampaign("link_id must be a dongeng UUID")
		}
		var count int64
		if err := s.db.Model(&models.Dongeng{}).Where("id = ? AND is_deleted = false", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return invalidCampaign("dongeng %q not found", req.LinkID)
		}
	default:
		return invalidCampaign("link_type must be one of none, ar_card, dongeng")
	}
	return nil
}

// List returns a page of campaigns, newest first.
func (s *AdminCampaignService) List(page, perPage int) ([]models.Campaign, int64, error) {
	var items []models.Campaign
	var total int64
	q := s.db.Model(&models.Campaign{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").Limit(perPage).Offset((page - 1) * perPage).Find(&items).Error
	return items, total, err
}

func (s *AdminCampaignService) pushMessage(c models.Campaign) PushMessage {
	return PushMessage{
		Title:            c.Title,
		Body:             c.Body,
		ImageURL:         c.ImageURL,
		AndroidChannelID: AndroidChannelPromo,
		Data: map[string]string{
			"type":        "campaign",
			"campaign_id": c.ID.String(),
			"link_type":   c.LinkType,
			"link_id":     c.LinkID,
		},
	}
}

// deliver sends the campaign and records the outcome on its row. sent/failed
// count individual deliveries: device pushes and emails for user segments,
// or the single topic message for all_devices.
func (s *AdminCampaignService) deliver(c models.Campaign) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("campaign: delivery panicked", "campaign_id", c.ID, "panic", r)
			s.finish(c.ID, models.CampaignStatusFailed, fmt.Sprint(r))
		}
	}()

	msg := s.pushMessage(c)

	if c.Segment == models.CampaignSegmentAllDevices {
		if err := s.notificationSvc.SendToTopic(PromoTopic, msg); err != nil {
			slog.Error("campaign: topic send failed", "campaign_id", c.ID, "error", err)
			s.setCounts(c.ID, 0, 1)
			s.finish(c.ID, models.CampaignStatusFailed, err.Error())
			return
		}
		s.setCounts(c.ID, 1, 0)
		s.finish(c.ID, models.CampaignStatusCompleted, "")
		return
	}

	users, err := s.resolveSegment(c.Segment)
	if err != nil {
		slog.Error("campaign: resolve segment failed", "campaign_id", c.ID, "error", err)
		s.finish(c.ID, models.CampaignStatusFailed, err.Error())
		return
	}

	sendPush := c.Channel == "push" || c.Channel == "both"
	sendEmail := c.Channel == "email" || c.Channel == "both"
	var sent, failed int

	for start := 0; start < len(users); start += campaignChunkSize {
		end := start + campaignChunkSize
		if end > len(users) {
			end = len(users)
		}

		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, campaignConcurrency)
		for _, u := range users[start:end] {
			wg.Add(1)
			sem <- struct{}{}
			go func(u segmentUser) {
				defer func() { <-sem; wg.Done() }()
				var ok, bad int
				if sendPush {
					d, f, err := s.notificationSvc.SendToUser(u.ID, "campaign", msg)
					if err != nil {
						f++
					}
					ok, bad = ok+d, bad+f
				}
				if sendEmail && u.EmailAddress != "" {
					if err := sendCampaignEmail(u.EmailAddress, c.Title, c.Body); err != nil {
						slog.Warn("campaign: email send failed", "user_id", u.ID, "error", err)
						bad++
					} else {
						ok++
					}
				}
				mu.Lock()
				sent, failed = sent+ok, failed+bad
				mu.Unlock()
			}(u)
		}
		wg.Wait()

		s.setCounts(c.ID, sent, failed)
		// Small pause between chunks to respect FCM/SMTP rate limits.
		if end < len(users) {
			time.Sleep(100 * time.Millisecond)
		}
	}
	s.finish(c.ID, models.CampaignStatusCompleted, "")
}

func (s *AdminCampaignService) setCounts(id uuid.UUID, sent, failed int) {
	if err := s.db.Model(&models.Campaign{}).Where("id = ?", id).
		Updates(map[string]interface{}{"sent": sent, "failed": failed}).Error; err != nil {
		slog.Warn("campaign: failed to record progress", "campaign_id", id, "error", err)
	}
}

func (s *AdminCampaignService) finish(id uuid.UUID, status, errMsg string) {
	if err := s.db.Model(&models.Campaign{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "error": errMsg, "completed_at": time.Now()}).Error; err != nil {
		slog.Error("campaign: failed to record completion", "campaign_id", id, "error", err)
	}
}

type segmentUser struct {
	ID           uuid.UUID
	EmailAddress string
}

func (s *AdminCampaignService) resolveSegment(segment string) ([]segmentUser, error) {
	var users []segmentUser
	q := s.db.Table("parents p").Select("p.id AS id, p.email_address AS email_address").Where("p.is_deleted = false")
	switch segment {
	case models.CampaignSegmentSubscribers:
		q = q.Joins("JOIN user_subscriptions us ON us.user_id = p.id").
			Where("us.status = 'premium' AND (us.expires_at IS NULL OR us.expires_at > NOW())")
	case models.CampaignSegmentAll:
	default:
		return nil, errors.New("unsupported segment " + segment)
	}
	err := q.Scan(&users).Error
	return users, err
}

// sendCampaignEmail sends a simple HTML email campaign message.
func sendCampaignEmail(to, subject, body string) error {
	htmlBody := fmt.Sprintf(`<html><body><h2>%s</h2><p>%s</p></body></html>`,
		html.EscapeString(subject), strings.ReplaceAll(html.EscapeString(body), "\n", "<br>"))
	return SendGenericEmail(to, subject, htmlBody)
}
