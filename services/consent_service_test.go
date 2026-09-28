package services

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const isRequiredQuery = `SELECT DISTINCT ON (document) document, version`

func consentRows(versions map[string]string) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"document", "version"})
	for doc, v := range versions {
		rows.AddRow(doc, v)
	}
	return rows
}

func currentVersions() map[string]string {
	out := map[string]string{}
	for doc, v := range CurrentConsentVersions {
		out[doc] = v
	}
	return out
}

func TestConsentInput_Rows_RecordsEveryDocument(t *testing.T) {
	userID := uuid.New()
	rows, err := ConsentInput{TermsVersion: "2026-10-01", PrivacyVersion: "2026-10-01", ParentalVersion: "2026-09-01"}.
		Rows(userID, "1.2.3.4", "Arunika/1.0")

	require.NoError(t, err)
	require.Len(t, rows, 3)
	byDoc := map[string]string{}
	for _, r := range rows {
		byDoc[r.Document] = r.Version
		assert.Equal(t, userID, r.UserID)
		assert.Equal(t, "1.2.3.4", r.IPAddress)
		assert.Equal(t, "Arunika/1.0", r.UserAgent)
	}
	// The version stored is the one the client sent, even when outdated.
	assert.Equal(t, map[string]string{"TERMS": "2026-10-01", "PRIVACY": "2026-10-01", "PARENTAL": "2026-09-01"}, byDoc)
}

func TestConsentInput_Rows_RejectsMissingVersion(t *testing.T) {
	_, err := ConsentInput{TermsVersion: "2026-10-01", PrivacyVersion: "2026-10-01", ParentalVersion: "  "}.
		Rows(uuid.New(), "", "")

	assert.ErrorIs(t, err, ErrConsentIncomplete)
}

func TestConsentService_IsRequired_CurrentVersionsAccepted(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewConsentService(gormDB)
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(isRequiredQuery)).WithArgs(userID).
		WillReturnRows(consentRows(currentVersions()))

	required, err := svc.IsRequired(userID)

	require.NoError(t, err)
	assert.False(t, required)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConsentService_IsRequired_OutdatedVersion(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewConsentService(gormDB)
	userID := uuid.New()
	accepted := currentVersions()
	accepted["PRIVACY"] = "2020-01-01"

	mock.ExpectQuery(regexp.QuoteMeta(isRequiredQuery)).WithArgs(userID).
		WillReturnRows(consentRows(accepted))

	required, err := svc.IsRequired(userID)

	require.NoError(t, err)
	assert.True(t, required)
}

func TestConsentService_IsRequired_NoConsentOnRecord(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewConsentService(gormDB)
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(isRequiredQuery)).WithArgs(userID).
		WillReturnRows(consentRows(nil))

	required, err := svc.IsRequired(userID)

	require.NoError(t, err)
	assert.True(t, required, "a legacy account with no consent rows must be asked")
}

func TestConsentService_IsRequired_MissingOneDocument(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewConsentService(gormDB)
	userID := uuid.New()
	accepted := currentVersions()
	delete(accepted, "PARENTAL")

	mock.ExpectQuery(regexp.QuoteMeta(isRequiredQuery)).WithArgs(userID).
		WillReturnRows(consentRows(accepted))

	required, err := svc.IsRequired(userID)

	require.NoError(t, err)
	assert.True(t, required)
}

func TestConsentService_Record_AppendsRows(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewConsentService(gormDB)
	userID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_consents"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).
			AddRow(uuid.New()).AddRow(uuid.New()).AddRow(uuid.New()))
	mock.ExpectCommit()

	err := svc.Record(userID, ConsentInput{
		TermsVersion: "2026-10-01", PrivacyVersion: "2026-10-01", ParentalVersion: "2026-10-01",
	}, "1.2.3.4", "ua")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConsentService_Record_IncompleteWritesNothing(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewConsentService(gormDB)

	err := svc.Record(uuid.New(), ConsentInput{TermsVersion: "2026-10-01"}, "", "")

	assert.ErrorIs(t, err, ErrConsentIncomplete)
	assert.NoError(t, mock.ExpectationsWereMet())
}
