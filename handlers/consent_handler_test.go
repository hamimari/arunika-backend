package handlers

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"arunika_backend/services"
)

const signupBodyTemplate = `{
	"name": "New User",
	"phone_number": "081",
	"email_address": "new@example.com",
	"address": "Jl.",
	"city": "Jakarta",
	"password": "secret123",
	"child": [{"name": "Budi", "gender": "M", "date_of_birth": "2020-01-02T00:00:00.000"}]%s
}`

func postSignup(h *AuthHandler, consentJSON string) *httptest.ResponseRecorder {
	body := strings.Replace(signupBodyTemplate, "%s", consentJSON, 1)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/signup", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "Arunika/1.0")
	h.SignUp(c)
	return w
}

// A consent object missing a version is rejected before anything is written:
// no DB expectations are set, so any query would fail the test.
func TestAuthHandler_SignUp_IncompleteConsent_Returns400(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewAuthHandler(services.NewAuthService(gormDB, nil))

	w := postSignup(h, `, "consent": {"terms_version": "2026-10-01", "privacy_version": "2026-10-01"}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "parental_version")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// With consent, the rows are inserted together with the parent in the same
// transaction. JWT_SECRET is blanked so the response is a 500 after the
// account commits; that is beside the point here, the insert is what's under
// test.
func TestAuthHandler_SignUp_WithConsent_InsertsConsentRowsWithAccount(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	gormDB, mock := setupHandlerDB(t)
	h := NewAuthHandler(services.NewAuthService(gormDB, nil))

	parentCols := []string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1`)).
		WillReturnRows(sqlmock.NewRows(parentCols))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE phone_number = $1`)).
		WillReturnRows(sqlmock.NewRows(parentCols))
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "parents"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "children"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_consents"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(uuid.New()).AddRow(uuid.New()).AddRow(uuid.New()))
	mock.ExpectCommit()

	postSignup(h, `, "consent": {"terms_version": "2026-10-01", "privacy_version": "2026-10-01", "parental_version": "2026-10-01"}`)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func postConsent(h *UserHandler, userID, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/user/consent", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)
	h.RecordConsent(c)
	return w
}

func TestUserHandler_RecordConsent_ClearsRequirement(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	cs := services.NewConsentService(gormDB)
	h := NewUserHandler(nil, nil, nil).WithConsent(cs)
	userID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_consents"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(uuid.New()).AddRow(uuid.New()).AddRow(uuid.New()))
	mock.ExpectCommit()
	latest := sqlmock.NewRows([]string{"document", "version"})
	for doc, v := range services.CurrentConsentVersions {
		latest.AddRow(doc, v)
	}
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT DISTINCT ON (document) document, version`)).
		WithArgs(userID).WillReturnRows(latest)

	w := postConsent(h, userID.String(), `{"terms_version": "2026-10-01", "privacy_version": "2026-10-01", "parental_version": "2026-10-01"}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"consent_required": false}`, w.Body.String())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserHandler_RecordConsent_IncompleteReturns400(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewUserHandler(nil, nil, nil).WithConsent(services.NewConsentService(gormDB))

	w := postConsent(h, uuid.New().String(), `{"terms_version": "2026-10-01"}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserHandler_GetUserByID_ReportsConsentRequired(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewUserHandler(services.NewUserService(gormDB), nil, nil).
		WithConsent(services.NewConsentService(gormDB))
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(userID, "Jane"))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "children"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	// A legacy account: nothing accepted yet.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT DISTINCT ON (document) document, version`)).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"document", "version"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/user/"+userID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: userID.String()}}
	c.Set("userID", userID.String())

	h.GetUserByID(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"consent_required":true`)
	assert.NoError(t, mock.ExpectationsWereMet())
}
