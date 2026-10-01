package services

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/utils"
)

// parentRowCols mirrors the columns GORM selects for models.Parent.
var parentRowCols = []string{
	"id", "name", "phone_number", "email_address", "password",
	"address", "city", "created_at", "updated_at", "is_deleted", "email_verified",
}

func TestIssueEmailVerificationToken_StoresHashNotRawToken(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)
	userID := uuid.New()

	// Prior tokens are cleared so only the newest emailed link works.
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "email_verification_tokens" WHERE user_id = $1`)).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	var storedToken string
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "email_verification_tokens"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	rawToken, err := svc.IssueEmailVerificationToken(userID)
	require.NoError(t, err)
	require.NotEmpty(t, rawToken)

	// The value handed back for emailing must never be what is persisted.
	storedToken = utils.HashToken(rawToken)
	assert.NotEqual(t, rawToken, storedToken)
	assert.Len(t, storedToken, 64, "a SHA-256 hex digest is 64 characters")
}

func TestConsumeEmailVerificationToken_EmptyToken_IsInvalid(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	err := svc.ConsumeEmailVerificationToken("")

	assert.ErrorIs(t, err, ErrVerificationTokenInvalid)
}

func TestConsumeEmailVerificationToken_UnknownToken_IsInvalid(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "email_verification_tokens" WHERE token = $1`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at"}))

	err := svc.ConsumeEmailVerificationToken("never-issued")

	assert.ErrorIs(t, err, ErrVerificationTokenInvalid)
}

func TestConsumeEmailVerificationToken_ExpiredToken_IsRejectedAndCleared(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	userID := uuid.New()
	raw := "expired-token"

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "email_verification_tokens" WHERE token = $1`)).
		WithArgs(utils.HashToken(raw), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at"}).
			AddRow(uuid.New(), userID, utils.HashToken(raw), time.Now().Add(-time.Hour)))

	// Account is still unverified, so expiry is what rejects it.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1`)).
		WillReturnRows(sqlmock.NewRows(parentRowCols).
			AddRow(userID, "Budi", "081", "b@example.com", "hash", "Jl.", "Jakarta",
				time.Now(), time.Now(), false, false))

	// The dead token is cleared so a resend starts from a clean slate.
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "email_verification_tokens" WHERE user_id = $1`)).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.ConsumeEmailVerificationToken(raw)

	assert.ErrorIs(t, err, ErrVerificationTokenInvalid)
}

func TestConsumeEmailVerificationToken_AlreadyVerified_ReportsAlreadyVerified(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	userID := uuid.New()
	raw := "still-valid"

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "email_verification_tokens" WHERE token = $1`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at"}).
			AddRow(uuid.New(), userID, utils.HashToken(raw), time.Now().Add(time.Hour)))

	// email_verified is already true.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1`)).
		WillReturnRows(sqlmock.NewRows(parentRowCols).
			AddRow(userID, "Budi", "081", "b@example.com", "hash", "Jl.", "Jakarta",
				time.Now(), time.Now(), false, true))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "email_verification_tokens" WHERE user_id = $1`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	name, err := svc.ConsumeEmailVerificationTokenForPage(raw)

	// Already-verified is a success for the user, not a failure.
	assert.ErrorIs(t, err, ErrEmailAlreadyVerified)
	assert.Equal(t, "Budi", name)
}

func TestResendVerificationEmail_AlreadyVerified_IssuesNoToken(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1`)).
		WillReturnRows(sqlmock.NewRows(parentRowCols).
			AddRow(userID, "Budi", "081", "b@example.com", "hash", "Jl.", "Jakarta",
				time.Now(), time.Now(), false, true))

	err := svc.ResendVerificationEmail(userID)

	assert.ErrorIs(t, err, ErrEmailAlreadyVerified)
	// No INSERT was expected; if one had been issued this would fail.
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIsEmailVerified_ReportsStoredValue(t *testing.T) {
	for _, tc := range []struct {
		name     string
		verified bool
	}{
		{"verified account", true},
		{"unverified account", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gormDB, mock := setupMockDB(t)
			svc := NewAuthService(gormDB, nil)
			userID := uuid.New()

			mock.ExpectQuery(regexp.QuoteMeta(`SELECT "email_verified" FROM "parents" WHERE id = $1`)).
				WillReturnRows(sqlmock.NewRows([]string{"email_verified"}).AddRow(tc.verified))

			got, err := svc.IsEmailVerified(userID)

			require.NoError(t, err)
			assert.Equal(t, tc.verified, got)
		})
	}
}

// ─── Password-reset gate ──────────────────────────────────────────────────────

// The reset gate is the one user-blocking behaviour email verification adds,
// and the easiest thing in it to get wrong: if the unverified branch reported
// a different outcome from the unknown-email branch, this endpoint would
// become an oracle for which addresses exist and which are verified.
//
// ForgotPassword returns nil for "no such account" by deliberate design (see
// its doc comment and TestForgotPassword_UnknownEmail_ReturnsNilWithoutEnumeration).
// The unverified branch must be byte-identical to it.
func TestForgotPassword_UnverifiedEmail_IsIndistinguishableFromUnknownEmail(t *testing.T) {
	now := time.Now()

	// Case 1: the address has no account at all.
	unknownDB, unknownMock := setupMockDB(t)
	unknownMock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1`)).
		WillReturnRows(sqlmock.NewRows(parentRowCols))
	unknownErr := NewAuthService(unknownDB, nil).ForgotPassword("nobody@example.com")

	// Case 2: the account exists but its address was never verified.
	unverifiedDB, unverifiedMock := setupMockDB(t)
	unverifiedMock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1`)).
		WillReturnRows(sqlmock.NewRows(parentRowCols).
			AddRow(uuid.New(), "Budi", "081", "budi@example.com", "hash", "Jl.", "Jakarta",
				now, now, false, false))
	unverifiedErr := NewAuthService(unverifiedDB, nil).ForgotPassword("budi@example.com")

	// Indistinguishable to the caller.
	assert.NoError(t, unknownErr)
	assert.NoError(t, unverifiedErr)
	assert.Equal(t, unknownErr, unverifiedErr,
		"an unverified address must be indistinguishable from an unknown one")

	// And crucially: no reset token was created for the unverified account.
	// Neither mock has an ExpectBegin/INSERT, so an attempt would fail here.
	require.NoError(t, unknownMock.ExpectationsWereMet())
	require.NoError(t, unverifiedMock.ExpectationsWereMet())
}

// The counterpart: a verified account still gets its reset link, so the gate
// has not simply disabled password recovery for everyone.
func TestForgotPassword_VerifiedEmail_ProceedsToIssueToken(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)
	useUnreachableSMTP(t)

	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1`)).
		WillReturnRows(sqlmock.NewRows(parentRowCols).
			AddRow(userID, "Budi", "081", "budi@example.com", "hash", "Jl.", "Jakarta",
				now, now, false, true))

	// Reaching the token machinery at all is the point of this test.
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "password_reset_tokens" WHERE user_id = $1`)).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "password_reset_tokens"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	_ = svc.ForgotPassword("budi@example.com")

	// The SMTP send fails (useUnreachableSMTP), but the reset token was
	// issued — which is what the gate governs.
	require.NoError(t, mock.ExpectationsWereMet())
}

// Regression guard inherited from the OTP weak-randomness finding (F6).
//
// The original guard lived on generateOtp, which seeded math/rand from the
// wall clock and was deleted along with the rest of the dead OTP flow. The
// verification token is now the security-sensitive value this codebase
// generates, so the guard belongs here.
//
// This cannot prove unpredictability — no test can — but it does catch a
// regression to a low-entropy source: a clock-seeded generator called in a
// tight loop collides readily, whereas crypto/rand-backed UUIDv4 values
// should never collide across a small sample.
func TestIssueEmailVerificationToken_DrawsFromAHighEntropySource(t *testing.T) {
	const draws = 25

	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)
	userID := uuid.New()

	for i := 0; i < draws; i++ {
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "email_verification_tokens" WHERE user_id = $1`)).
			WithArgs(userID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "email_verification_tokens"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
		mock.ExpectCommit()
	}

	rawTokens := make(map[string]bool, draws)
	hashes := make(map[string]bool, draws)
	for i := 0; i < draws; i++ {
		raw, err := svc.IssueEmailVerificationToken(userID)
		require.NoError(t, err)
		require.NotEmpty(t, raw)
		rawTokens[raw] = true
		hashes[utils.HashToken(raw)] = true
	}

	assert.Len(t, rawTokens, draws,
		"every issued token must be distinct; collisions indicate a low-entropy source")
	assert.Len(t, hashes, draws,
		"distinct tokens must hash to distinct values")
	require.NoError(t, mock.ExpectationsWereMet())
}
