package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// fakeFCM points the notification service at an httptest server with a
// static OAuth token, returning the captured request bodies.
func fakeFCM(t *testing.T, ns *NotificationService, status int) *[]map[string]interface{} {
	t.Helper()
	var bodies []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	const saJSON = `{"project_id":"test-project"}`
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", saJSON)
	ns.credJSON = saJSON
	ns.projectID = "test-project"
	ns.tokenSource = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test"})

	orig := fcmEndpoint
	fcmEndpoint = func(string) string { return srv.URL }
	t.Cleanup(func() { fcmEndpoint = orig })
	return &bodies
}

func newSyncCampaignService(t *testing.T) (*AdminCampaignService, sqlmock.Sqlmock) {
	db, mock := setupMockDB(t)
	svc := NewAdminCampaignService(db, NewNotificationService(db))
	svc.runAsync = func(f func()) { f() }
	return svc, mock
}

func TestCampaignDispatch_PushWithoutUsableCredentialsIsRejectedUpFront(t *testing.T) {
	svc, mock := newSyncCampaignService(t)
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "/path/to/firebase-service-account.json")

	_, err := svc.Dispatch(CampaignRequest{Title: "t", Body: "b", Channel: "push", Segment: "all_devices"}, nil)

	assert.ErrorIs(t, err, ErrPushUnavailable)
	assert.Contains(t, err.Error(), "neither JSON nor a readable file path")
	assert.NoError(t, mock.ExpectationsWereMet(), "no campaign row should be created")
}

func TestCampaignDispatch_AllDevicesRequiresPush(t *testing.T) {
	svc, _ := newSyncCampaignService(t)
	_, err := svc.Dispatch(CampaignRequest{Title: "t", Body: "b", Channel: "both", Segment: "all_devices"}, nil)
	var vErr *CampaignValidationError
	assert.ErrorAs(t, err, &vErr)
}

func TestCampaignDispatch_RejectsUnknownLinkTarget(t *testing.T) {
	svc, mock := newSyncCampaignService(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "ar_cards" WHERE id = $1`)).
		WithArgs("card-x").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	_, err := svc.Dispatch(CampaignRequest{
		Title: "t", Body: "b", Channel: "push", Segment: "all_devices",
		LinkType: "ar_card", LinkID: "card-x",
	}, nil)
	var vErr *CampaignValidationError
	assert.ErrorAs(t, err, &vErr)
}

func TestCampaignDispatch_RejectsNonHTTPSImage(t *testing.T) {
	svc, _ := newSyncCampaignService(t)
	_, err := svc.Dispatch(CampaignRequest{
		Title: "t", Body: "b", Channel: "push", Segment: "all_devices", ImageURL: "http://x/y.png",
	}, nil)
	var vErr *CampaignValidationError
	assert.ErrorAs(t, err, &vErr)
}

func TestCampaignDispatch_AllDevicesSendsToTopic(t *testing.T) {
	svc, mock := newSyncCampaignService(t)
	bodies := fakeFCM(t, svc.notificationSvc, http.StatusOK)
	campaignID := uuid.New()
	dongengID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE id = $1 AND is_deleted = false`)).
		WithArgs(dongengID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "campaigns"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(campaignID))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "campaigns" SET "failed"=$1,"sent"=$2 WHERE id = $3`)).
		WithArgs(0, 1, campaignID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "campaigns" SET "completed_at"=$1,"error"=$2,"status"=$3 WHERE id = $4`)).
		WithArgs(sqlmock.AnyArg(), "", "COMPLETED", campaignID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	campaign, err := svc.Dispatch(CampaignRequest{
		Title: "Dongeng baru!", Body: "Kancil dan Buaya sudah rilis", Channel: "push", Segment: "all_devices",
		ImageURL: "https://cdn.example.com/kancil.png", LinkType: "dongeng", LinkID: dongengID.String(),
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "SENDING", campaign.Status)
	assert.NoError(t, mock.ExpectationsWereMet())

	require.Len(t, *bodies, 1)
	message := (*bodies)[0]["message"].(map[string]interface{})
	assert.Equal(t, PromoTopic, message["topic"])
	assert.Equal(t, "https://cdn.example.com/kancil.png", message["notification"].(map[string]interface{})["image"])
	android := message["android"].(map[string]interface{})
	assert.Equal(t, "high", android["priority"])
	assert.Equal(t, AndroidChannelPromo, android["notification"].(map[string]interface{})["channel_id"])
	data := message["data"].(map[string]interface{})
	assert.Equal(t, "dongeng", data["link_type"])
	assert.Equal(t, dongengID.String(), data["link_id"])
	assert.Equal(t, campaignID.String(), data["campaign_id"])
}

func TestCampaignDispatch_TopicFailureMarksCampaignFailed(t *testing.T) {
	svc, mock := newSyncCampaignService(t)
	fakeFCM(t, svc.notificationSvc, http.StatusInternalServerError)
	campaignID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "campaigns"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(campaignID))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "campaigns" SET "failed"=$1,"sent"=$2 WHERE id = $3`)).
		WithArgs(1, 0, campaignID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "campaigns" SET "completed_at"=$1,"error"=$2,"status"=$3 WHERE id = $4`)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "FAILED", campaignID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, err := svc.Dispatch(CampaignRequest{Title: "t", Body: "b", Channel: "push", Segment: "all_devices"}, nil)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
