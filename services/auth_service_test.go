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
