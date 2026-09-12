package services

import (
	"context"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"arunika_backend/models"
	"arunika_backend/utils"
)

// TestValidateCredentials_Success tests that valid email + password passes authentication.
func TestValidateCredentials_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.MinCost)
	parentID := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(parentID, "John Doe", "08123456789", "john@example.com", string(hashedPwd),
		"Jl. Test", "Jakarta", now, now, false)

	// GORM's First() adds ORDER BY primary key and LIMIT 1
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("john@example.com", 1).
		WillReturnRows(rows)

	user, err := svc.ValidateCredentials("john@example.com", "correctpassword")

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "john@example.com", user.EmailAddress)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestValidateCredentials_WrongPassword tests that incorrect password returns an error.
func TestValidateCredentials_WrongPassword(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.MinCost)
	parentID := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(parentID, "John Doe", "08123456789", "john@example.com", string(hashedPwd),
		"Jl. Test", "Jakarta", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("john@example.com", 1).
		WillReturnRows(rows)

	user, err := svc.ValidateCredentials("john@example.com", "wrongpassword")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Equal(t, "invalid credential", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestValidateCredentials_UserNotFound tests that a non-existent email returns an error.
func TestValidateCredentials_UserNotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	emptyRows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	})

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("unknown@example.com", 1).
		WillReturnRows(emptyRows)

	user, err := svc.ValidateCredentials("unknown@example.com", "anypassword")

	assert.Error(t, err)
	assert.Nil(t, user)
}

// TestValidateRefreshToken_Valid tests that a valid, non-expired refresh token is accepted.
func TestValidateRefreshToken_Valid(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	userID := uuid.New().String()
	tokenVal := uuid.New().String()
	tokenID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(7 * 24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "token", "expires_at", "created_at", "updated_at", "is_deleted",
	}).AddRow(tokenID, userID, tokenVal, expiresAt, now, now, false)

	// FindUserUserIdAndToken: WHERE token = $1 and user_id = $2 ... LIMIT $3
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "refresh_tokens" WHERE token = $1 and user_id = $2 ORDER BY "refresh_tokens"."id" LIMIT $3`)).
		WithArgs(tokenVal, userID, 1).
		WillReturnRows(rows)

	resultUserID, err := svc.ValidateRefreshToken(userID, tokenVal)

	require.NoError(t, err)
	assert.Equal(t, userID, resultUserID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestValidateRefreshToken_Expired tests that an expired refresh token is rejected.
func TestValidateRefreshToken_Expired(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	userID := uuid.New().String()
	tokenVal := uuid.New().String()
	tokenID := uuid.New()
	now := time.Now()
	expiredAt := now.Add(-1 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "token", "expires_at", "created_at", "updated_at", "is_deleted",
	}).AddRow(tokenID, userID, tokenVal, expiredAt, now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "refresh_tokens" WHERE token = $1 and user_id = $2 ORDER BY "refresh_tokens"."id" LIMIT $3`)).
		WithArgs(tokenVal, userID, 1).
		WillReturnRows(rows)

	_, err := svc.ValidateRefreshToken(userID, tokenVal)

	assert.Error(t, err)
	assert.Equal(t, "token expired", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGenerateJwtToken_StoresRefreshToken verifies the token is persisted and both tokens returned.
// GORM v2 with postgres uses RETURNING so Insert becomes a Query, not an Exec.
func TestGenerateJwtToken_StoresRefreshToken(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	os.Setenv("JWT_SECRET", "test-secret-key-at-least-32-chars!!")
	defer os.Unsetenv("JWT_SECRET")

	tokenID := uuid.New()
	returnRows := sqlmock.NewRows([]string{"id"}).AddRow(tokenID)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "refresh_tokens"`)).
		WillReturnRows(returnRows)
	mock.ExpectCommit()

	accessToken, refreshToken, err := svc.GenerateJwtToken("user-123", "test@example.com")

	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	svc := NewAuthService(gormDB, rdb)

	tokenVal := uuid.New().String()
	jti := uuid.New().String()
	exp := time.Now().Add(15 * time.Minute)

	// GORM hard-delete: DELETE FROM refresh_tokens WHERE token=?
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "refresh_tokens" WHERE token = $1`)).
		WithArgs(tokenVal).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)

	err = svc.Logout(ginCtx, tokenVal, jti, exp)

	require.NoError(t, err)
	// Redis blacklist key should exist
	val, err := rdb.Get(context.Background(), "blacklist:"+jti).Result()
	require.NoError(t, err)
	assert.Equal(t, "revoked", val)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── Signup ───────────────────────────────────────────────────────────────────

func TestSignup_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	// FindUserByEmail → empty (email not taken)
	emptyRows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	})
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("new@example.com", 1).
		WillReturnRows(emptyRows)

	// FindUserByPhoneNumber → empty (phone not taken)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE phone_number = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("081", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted",
		}))

	// db.Create(&request) — GORM v2 postgres uses RETURNING
	newID := uuid.New()
	returnRows := sqlmock.NewRows([]string{"id"}).AddRow(newID)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "parents"`)).
		WillReturnRows(returnRows)
	mock.ExpectCommit()

	req := models.Parent{
		Name:         "New User",
		PhoneNumber:  "081",
		EmailAddress: "new@example.com",
		Password:     "hashed",
		Address:      "Jl.",
		City:         "Jakarta",
	}

	result, err := svc.Signup(req)

	require.NoError(t, err)
	assert.Equal(t, "new@example.com", result.EmailAddress)
}

func TestSignup_EmailAlreadyTaken(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	existingID := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(existingID, "Existing", "081", "taken@example.com", string(hashedPwd),
		"Jl.", "Jakarta", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("taken@example.com", 1).
		WillReturnRows(rows)

	req := models.Parent{EmailAddress: "taken@example.com"}
	_, err := svc.Signup(req)

	assert.Error(t, err)
	assert.Equal(t, "email address already taken", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSignup_PhoneAlreadyTaken(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	existingID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("new@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted",
		}))

	phoneRows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(existingID, "Existing", "081234", "existing@example.com", string(hashedPwd),
		"Jl.", "Jakarta", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE phone_number = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("081234", 1).
		WillReturnRows(phoneRows)

	req := models.Parent{EmailAddress: "new@example.com", PhoneNumber: "081234"}
	_, err := svc.Signup(req)

	assert.Error(t, err)
	assert.Equal(t, "phone number already taken", err.Error())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── CheckAvailability ──────────────────────────────────────────────────────

func TestCheckAvailability_BothAvailable(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	emptyRows := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted",
		})
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("free@example.com", 1).
		WillReturnRows(emptyRows())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE phone_number = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("080000", 1).
		WillReturnRows(emptyRows())

	emailTaken, phoneTaken, err := svc.CheckAvailability("free@example.com", "080000")

	require.NoError(t, err)
	assert.False(t, emailTaken)
	assert.False(t, phoneTaken)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAvailability_BothTaken(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	now := time.Now()
	takenRow := func(email, phone string) *sqlmock.Rows {
		return sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted",
		}).AddRow(uuid.New(), "Existing", phone, email, "hash", "Jl.", "Jakarta", now, now, false)
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("taken@example.com", 1).
		WillReturnRows(takenRow("taken@example.com", "081999"))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE phone_number = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("081999", 1).
		WillReturnRows(takenRow("taken@example.com", "081999"))

	emailTaken, phoneTaken, err := svc.CheckAvailability("taken@example.com", "081999")

	require.NoError(t, err)
	assert.True(t, emailTaken)
	assert.True(t, phoneTaken)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckAvailability_EmptyInputsSkipped(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	emailTaken, phoneTaken, err := svc.CheckAvailability("", "")

	require.NoError(t, err)
	assert.False(t, emailTaken)
	assert.False(t, phoneTaken)
}

// ─── ForgotPassword / ResetPassword ────────────────────────────────────────────

// Points SendEmail at a closed local port so it fails fast (connection
// refused) instead of trying a real SMTP server or hanging.
func useUnreachableSMTP(t *testing.T) {
	t.Helper()
	os.Setenv("SMTP_HOST", "127.0.0.1")
	os.Setenv("SMTP_PORT", "1")
	os.Setenv("SMTP_USER", "test@example.com")
	os.Setenv("SMTP_PASSWORD", "unused")
	t.Cleanup(func() {
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASSWORD")
	})
}

func TestForgotPassword_UnknownEmail_ReturnsNilWithoutEnumeration(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("nobody@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted",
		}))

	err := svc.ForgotPassword("nobody@example.com")

	// Must return nil (not "user not found") — the caller must not be able
	// to tell an unknown email apart from a real send.
	require.NoError(t, err)
	// No token/email side effects — the mock has no other expectations set,
	// so this also proves no reset token was created for a nonexistent user.
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestForgotPassword_KnownEmail_InvalidatesOldTokensAndCreatesNewOne(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)
	useUnreachableSMTP(t)

	userID := uuid.New()
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("known@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted",
		}).AddRow(userID, "Known User", "081", "known@example.com", "hash", "Jl.", "Jakarta", now, now, false))

	// Old, still-valid tokens for this user are invalidated first.
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "password_reset_tokens" WHERE user_id = $1`)).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	// A new token row is created — the exact hashed value is covered by
	// TestHashResetToken_* and TestVerifyResetToken_HashesBeforeLookup; here
	// we only assert that a row is inserted.
	newID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "password_reset_tokens"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(newID))
	mock.ExpectCommit()

	// SendEmail will fail (unreachable SMTP) — ForgotPassword surfaces that
	// error so AuthHandler can log it, but never turns it into a different
	// client-visible outcome (see TestAuthHandler_ForgotPassword_* ).
	err := svc.ForgotPassword("known@example.com")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestForgotPassword_OldTokenInvalidationFailure_IsReturned(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	userID := uuid.New()
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE email_address = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("known@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted",
		}).AddRow(userID, "Known User", "081", "known@example.com", "hash", "Jl.", "Jakarta", now, now, false))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "password_reset_tokens" WHERE user_id = $1`)).
		WithArgs(userID).
		WillReturnError(assert.AnError)
	mock.ExpectRollback()

	err := svc.ForgotPassword("known@example.com")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyResetToken_HashesBeforeLookup(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	rawToken := "plain-text-token-from-the-email-link"
	rowID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "password_reset_tokens" WHERE token = $1 ORDER BY "password_reset_tokens"."id" LIMIT $2`)).
		WithArgs(utils.HashResetToken(rawToken), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at", "created_at", "updated_at", "is_deleted"}).
			AddRow(rowID, userID, utils.HashResetToken(rawToken), now.Add(10*time.Minute), now, now, false))

	reset, err := svc.VerifyResetToken(rawToken)

	require.NoError(t, err)
	require.NotNil(t, reset)
	assert.Equal(t, userID, reset.UserID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyResetToken_Expired(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	rawToken := "expired-token"
	rowID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "password_reset_tokens" WHERE token = $1 ORDER BY "password_reset_tokens"."id" LIMIT $2`)).
		WithArgs(utils.HashResetToken(rawToken), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at", "created_at", "updated_at", "is_deleted"}).
			AddRow(rowID, userID, utils.HashResetToken(rawToken), now.Add(-1*time.Minute), now, now, false))

	reset, err := svc.VerifyResetToken(rawToken)

	assert.Error(t, err)
	assert.Nil(t, reset)
	assert.Equal(t, "token expired", err.Error())
}

func TestVerifyResetToken_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "password_reset_tokens" WHERE token = $1 ORDER BY "password_reset_tokens"."id" LIMIT $2`)).
		WithArgs(utils.HashResetToken("unknown-token"), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at", "created_at", "updated_at", "is_deleted"}))

	reset, err := svc.VerifyResetToken("unknown-token")

	assert.Error(t, err)
	assert.Nil(t, reset)
	assert.Equal(t, "invalid token", err.Error())
}

func TestResetPassword_Success_RevokesRefreshTokens(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	rawToken := "valid-token"
	rowID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "password_reset_tokens" WHERE token = $1 ORDER BY "password_reset_tokens"."id" LIMIT $2`)).
		WithArgs(utils.HashResetToken(rawToken), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at", "created_at", "updated_at", "is_deleted"}).
			AddRow(rowID, userID, utils.HashResetToken(rawToken), now.Add(10*time.Minute), now, now, false))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE "parents"."id" = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "phone_number", "email_address", "password",
			"address", "city", "created_at", "updated_at", "is_deleted",
		}).AddRow(userID, "Known User", "081", "known@example.com", "oldhash", "Jl.", "Jakarta", now, now, false))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "parents" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "password_reset_tokens" WHERE "password_reset_tokens"."id" = $1`)).
		WithArgs(rowID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "refresh_tokens" WHERE user_id = $1`)).
		WithArgs(userID.String()).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()

	err := svc.ResetPassword(rawToken, "brand-new-password")

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResetPassword_ExpiredToken_Rejected(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAuthService(gormDB, nil)

	rawToken := "expired-token"
	rowID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "password_reset_tokens" WHERE token = $1 ORDER BY "password_reset_tokens"."id" LIMIT $2`)).
		WithArgs(utils.HashResetToken(rawToken), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at", "created_at", "updated_at", "is_deleted"}).
			AddRow(rowID, userID, utils.HashResetToken(rawToken), now.Add(-time.Minute), now, now, false))

	err := svc.ResetPassword(rawToken, "brand-new-password")

	assert.Error(t, err)
	// No password update, no delete, no revocation should have been attempted.
	assert.NoError(t, mock.ExpectationsWereMet())
}
