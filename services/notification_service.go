package services

import (
	"arunika_backend/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type NotificationService struct {
	db *gorm.DB
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{db: db}
}

// Send inserts a notification record and delivers FCM push to all user devices.
func (s *NotificationService) Send(userID uuid.UUID, title, body, notifType string) error {
	notif := models.Notification{
		UserID: userID,
		Title:  title,
		Body:   body,
		Type:   notifType,
	}
	if err := s.db.Create(&notif).Error; err != nil {
		slog.Error("NotificationService.Send: db insert", "error", err)
		return err
	}

	// Fetch all FCM tokens for user.
	var tokens []models.FCMToken
	if err := s.db.Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		slog.Error("NotificationService.Send: fetch tokens", "error", err)
		return nil // notification persisted; FCM is best-effort
	}

	for _, t := range tokens {
		if err := s.sendFCM(t.Token, title, body); err != nil {
			if isFCMNotFound(err) {
				slog.Info("NotificationService: stale FCM token, removing", "token", t.Token)
				s.db.Where("token = ?", t.Token).Delete(&models.FCMToken{})
			} else {
				slog.Warn("NotificationService.Send: FCM send failed", "token", t.Token, "error", err)
			}
		}
	}
	return nil
}

// RegisterToken upserts an FCM token for a user.
func (s *NotificationService) RegisterToken(userID uuid.UUID, token string) error {
	row := models.FCMToken{
		UserID:    userID,
		Token:     token,
		CreatedAt: time.Now(),
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id"}),
	}).Create(&row).Error
}

// GetNotifications returns notifications for a user ordered by newest first.
func (s *NotificationService) GetNotifications(userID uuid.UUID) ([]models.Notification, error) {
	var notifs []models.Notification
	err := s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&notifs).Error
	return notifs, err
}

// MarkRead marks a single notification as read.
func (s *NotificationService) MarkRead(userID uuid.UUID, notifID uuid.UUID) error {
	result := s.db.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notifID, userID).
		Update("is_read", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

// fcmNotFoundErr is a sentinel used to signal 404 from FCM.
type fcmNotFoundErr struct{}

func (e *fcmNotFoundErr) Error() string { return "fcm token not found" }

func isFCMNotFound(err error) bool {
	_, ok := err.(*fcmNotFoundErr)
	return ok
}

// sendFCM calls the FCM HTTP v1 API using a service account JSON for auth.
func (s *NotificationService) sendFCM(token, title, body string) error {
	saJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
	if saJSON == "" {
		return nil // FCM not configured; skip silently
	}

	ctx := context.Background()
	creds, err := google.CredentialsFromJSON(
		ctx,
		[]byte(saJSON),
		"https://www.googleapis.com/auth/firebase.messaging",
	)
	if err != nil {
		return fmt.Errorf("parse service account: %w", err)
	}

	tokenSource := creds.TokenSource
	oauthToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("get oauth token: %w", err)
	}

	// Extract project ID from SA JSON.
	var saMap map[string]interface{}
	json.Unmarshal([]byte(saJSON), &saMap)
	projectID, _ := saMap["project_id"].(string)
	if projectID == "" {
		return fmt.Errorf("project_id missing from service account JSON")
	}

	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"token": token,
			"notification": map[string]string{
				"title": title,
				"body":  body,
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+oauthToken.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &fcmNotFoundErr{}
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("FCM returned status %d", resp.StatusCode)
	}
	return nil
}
