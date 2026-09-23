package services

import (
	"arunika_backend/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// PromoTopic is the FCM topic every app install subscribes to (guests
// included), used for "all devices" promo campaigns.
const PromoTopic = "arunika_promo"

// AndroidChannelPromo is the app's high-importance Android notification
// channel for campaigns (created in MainActivity.kt), so promos pop up as
// heads-up notifications. Pushes without a channel use the app's default
// "arunika_updates" channel.
const AndroidChannelPromo = "arunika_promo"

// fcmEndpoint is a var so tests can point FCM sends at an httptest server.
var fcmEndpoint = func(projectID string) string {
	return fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID)
}

// PushMessage is the user-visible content of a push notification. Data is
// delivered to the app alongside it (FCM requires string values) and drives
// what the app opens when the notification is tapped.
type PushMessage struct {
	Title    string
	Body     string
	ImageURL string
	Data     map[string]string
	// AndroidChannelID selects the app's Android notification channel;
	// empty uses the app's default channel.
	AndroidChannelID string
}

type NotificationService struct {
	db         *gorm.DB
	httpClient *http.Client

	// FCM credentials are parsed once per distinct service-account JSON and
	// reused; the Google token source caches the OAuth token until expiry.
	credMu      sync.Mutex
	credJSON    string
	tokenSource oauth2.TokenSource
	projectID   string
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{db: db, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

// Send inserts a notification record and delivers FCM push to all user devices.
func (s *NotificationService) Send(userID uuid.UUID, title, body, notifType string) error {
	_, _, err := s.SendToUser(userID, notifType, PushMessage{Title: title, Body: body})
	return err
}

// SendToUser persists the notification for userID and pushes it to every
// device the user has registered. It reports how many device deliveries
// succeeded and failed; err is only non-nil when the notification could not
// be persisted (push delivery itself is best-effort).
func (s *NotificationService) SendToUser(userID uuid.UUID, notifType string, msg PushMessage) (delivered, failed int, err error) {
	notif := models.Notification{
		UserID: userID,
		Title:  msg.Title,
		Body:   msg.Body,
		Type:   notifType,
	}
	if err := s.db.Create(&notif).Error; err != nil {
		slog.Error("NotificationService.SendToUser: db insert", "error", err)
		return 0, 0, err
	}

	// Fetch all FCM tokens for user.
	var tokens []models.FCMToken
	if err := s.db.Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		slog.Error("NotificationService.SendToUser: fetch tokens", "error", err)
		return 0, 0, nil // notification persisted; FCM is best-effort
	}

	for _, t := range tokens {
		if s.sendToToken(t.Token, msg) {
			delivered++
		} else {
			failed++
		}
	}
	return delivered, failed, nil
}

// SendToTopic pushes msg to every device subscribed to topic. Nothing is
// persisted — topic audiences include guests with no user row.
func (s *NotificationService) SendToTopic(topic string, msg PushMessage) error {
	return s.sendFCM(map[string]interface{}{"topic": topic}, msg)
}

// sendToToken pushes to one device, pruning the token if FCM reports it is no
// longer registered. Returns whether delivery succeeded.
func (s *NotificationService) sendToToken(token string, msg PushMessage) bool {
	err := s.sendFCM(map[string]interface{}{"token": token}, msg)
	if err == nil {
		return true
	}
	if isFCMNotFound(err) {
		slog.Info("NotificationService: stale FCM token, removing", "token", token)
		s.db.Where("token = ?", token).Delete(&models.FCMToken{})
	} else {
		slog.Warn("NotificationService: FCM send failed", "token", token, "error", err)
	}
	return false
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

// ErrFCMNotConfigured is returned by topic sends when no service account is
// set — unlike per-user sends there is no persisted record to fall back on,
// so the caller needs to know nothing went out.
var ErrFCMNotConfigured = fmt.Errorf("FCM is not configured (FIREBASE_SERVICE_ACCOUNT_JSON is empty)")

// loadServiceAccountJSONFromEnv resolves envVar, which may hold either the
// raw service-account JSON or a path to the JSON file. Returns "" when the
// variable is unset. Shared by every Google service-account credential
// (FCM, Google Play Billing, ...) that follows this same convention.
func loadServiceAccountJSONFromEnv(envVar string) (string, error) {
	value := strings.TrimSpace(os.Getenv(envVar))
	if value == "" || strings.HasPrefix(value, "{") {
		return value, nil
	}
	content, err := os.ReadFile(value)
	if err != nil {
		return "", fmt.Errorf("%s is neither JSON nor a readable file path (%q): %w", envVar, value, err)
	}
	return string(content), nil
}

// CheckPushConfigured reports why push can't be sent (credentials unset or
// unusable), or nil when FCM is ready.
func (s *NotificationService) CheckPushConfigured() error {
	_, _, ok, err := s.fcmCredentials()
	if err != nil {
		return err
	}
	if !ok {
		return ErrFCMNotConfigured
	}
	return nil
}

// fcmCredentials returns a cached token source and project id for the
// service account in FIREBASE_SERVICE_ACCOUNT_JSON, or ok=false when unset.
func (s *NotificationService) fcmCredentials() (ts oauth2.TokenSource, projectID string, ok bool, err error) {
	saJSON, err := loadServiceAccountJSONFromEnv("FIREBASE_SERVICE_ACCOUNT_JSON")
	if err != nil {
		return nil, "", false, err
	}
	if saJSON == "" {
		return nil, "", false, nil
	}

	s.credMu.Lock()
	defer s.credMu.Unlock()
	if s.tokenSource != nil && s.credJSON == saJSON {
		return s.tokenSource, s.projectID, true, nil
	}

	creds, err := google.CredentialsFromJSON(
		context.Background(),
		[]byte(saJSON),
		"https://www.googleapis.com/auth/firebase.messaging",
	)
	if err != nil {
		return nil, "", false, fmt.Errorf("parse service account: %w", err)
	}
	var saMap map[string]interface{}
	_ = json.Unmarshal([]byte(saJSON), &saMap)
	projectID, _ = saMap["project_id"].(string)
	if projectID == "" {
		return nil, "", false, fmt.Errorf("project_id missing from service account JSON")
	}

	s.credJSON, s.tokenSource, s.projectID = saJSON, creds.TokenSource, projectID
	return s.tokenSource, s.projectID, true, nil
}

// sendFCM calls the FCM HTTP v1 API. target is {"token": ...} or {"topic": ...}.
func (s *NotificationService) sendFCM(target map[string]interface{}, msg PushMessage) error {
	tokenSource, projectID, ok, err := s.fcmCredentials()
	if err != nil {
		return err
	}
	if !ok {
		if _, isTopic := target["topic"]; isTopic {
			return ErrFCMNotConfigured
		}
		return nil // FCM not configured; skip silently
	}

	oauthToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("get oauth token: %w", err)
	}

	notification := map[string]string{
		"title": msg.Title,
		"body":  msg.Body,
	}
	if msg.ImageURL != "" {
		notification["image"] = msg.ImageURL
	}
	android := map[string]interface{}{"priority": "high"}
	if msg.AndroidChannelID != "" {
		android["notification"] = map[string]string{"channel_id": msg.AndroidChannelID}
	}
	message := map[string]interface{}{
		"notification": notification,
		"android":      android,
	}
	for k, v := range target {
		message[k] = v
	}
	if len(msg.Data) > 0 {
		message["data"] = msg.Data
	}
	payloadBytes, _ := json.Marshal(map[string]interface{}{"message": message})

	req, err := http.NewRequest("POST", fcmEndpoint(projectID), bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+oauthToken.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
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
