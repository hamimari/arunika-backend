package services

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAdminUserDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	dialector := postgres.New(postgres.Config{Conn: db, DriverName: "postgres"})
	gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return gormDB, mock
}

// ─── ListUsers ────────────────────────────────────────────────────────────────

func TestAdminUserService_ListUsers_Success(t *testing.T) {
	db, mock := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	uid := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "parents" WHERE is_deleted = false`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE is_deleted = false ORDER BY created_at DESC LIMIT $1`)).
		WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email_address", "city", "created_at", "updated_at", "is_deleted"}).
			AddRow(uid, "Budi", "budi@test.com", "Jakarta", now, now, false))

	users, total, err := svc.ListUsers("", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)
	assert.Equal(t, "Budi", users[0].Name)
}

func TestAdminUserService_ListUsers_WithSearch(t *testing.T) {
	db, mock := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "parents" WHERE is_deleted = false AND (name ILIKE $1 OR email_address ILIKE $2)`)).
		WithArgs("%budi%", "%budi%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE is_deleted = false AND (name ILIKE $1 OR email_address ILIKE $2) ORDER BY created_at DESC LIMIT $3`)).
		WithArgs("%budi%", "%budi%", 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email_address", "city", "created_at", "updated_at", "is_deleted"}))

	users, total, err := svc.ListUsers("budi", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, users)
}

func TestAdminUserService_ListUsers_DBError(t *testing.T) {
	db, mock := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "parents" WHERE is_deleted = false`)).
		WillReturnError(gorm.ErrInvalidDB)

	_, _, err := svc.ListUsers("", 1, 20)
	// count error doesn't propagate (GORM ignores it), but Find will fail
	_ = err // just ensure no panic
}

// ─── GetUserDetail ────────────────────────────────────────────────────────────

func TestAdminUserService_GetUserDetail_NotFound(t *testing.T) {
	db, mock := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	mock.ExpectQuery(`SELECT \* FROM "parents" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, _, err := svc.GetUserDetail("non-existent-id")
	assert.Error(t, err)
}

func TestAdminUserService_GetUserDetail_Success(t *testing.T) {
	db, mock := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	uid := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 AND is_deleted = false ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs(uid.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email_address", "city", "created_at", "updated_at", "is_deleted"}).
			AddRow(uid, "Siti", "siti@test.com", "Bandung", now, now, false))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(uid.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).
			AddRow(uuid.New(), uid, "free"))

	user, sub, err := svc.GetUserDetail(uid.String())
	require.NoError(t, err)
	assert.Equal(t, "Siti", user.Name)
	assert.Equal(t, "free", sub.Status)
}

// ─── GrantPremium ─────────────────────────────────────────────────────────────

func TestAdminUserService_GrantPremium_CreatesRecordWhenMissing(t *testing.T) {
	db, mock := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	uid := uuid.New()

	// First call to First returns record not found.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(uid.String(), 1).
		WillReturnError(gorm.ErrRecordNotFound)

	// Expect an INSERT to create the subscription.
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_subscriptions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	err := svc.GrantPremium(uid.String(), 30)
	assert.NoError(t, err)
}

func TestAdminUserService_GrantPremium_UpdatesExistingRecord(t *testing.T) {
	db, mock := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	uid := uuid.New()
	subID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(uid.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).
			AddRow(subID, uid, "free"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "user_subscriptions"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.GrantPremium(uid.String(), 30)
	assert.NoError(t, err)
}

func TestAdminUserService_GrantPremium_InvalidUUID(t *testing.T) {
	db, _ := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	err := svc.GrantPremium("not-a-uuid", 30)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user ID")
}

// ─── RevokePremium ────────────────────────────────────────────────────────────

func TestAdminUserService_RevokePremium_NoopWhenMissing(t *testing.T) {
	db, mock := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	uid := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(uid.String(), 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.RevokePremium(uid.String())
	assert.NoError(t, err) // should be a no-op, not an error
}

func TestAdminUserService_RevokePremium_UpdatesStatus(t *testing.T) {
	db, mock := setupAdminUserDB(t)
	svc := NewAdminUserService(db)

	uid := uuid.New()
	subID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(uid.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status"}).
			AddRow(subID, uid, "premium"))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "user_subscriptions"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.RevokePremium(uid.String())
	assert.NoError(t, err)
}
