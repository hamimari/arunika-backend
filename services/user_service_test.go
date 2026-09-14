package services

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserService_GetUserByID_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	userID := uuid.New()
	childID := uuid.New()
	now := time.Now()

	parentRows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(userID, "Alice", "081", "alice@example.com", "hash",
		"Jl. Test", "Jakarta", now, now, false)

	// GORM Preload("Children") executes a second query
	childRows := sqlmock.NewRows([]string{
		"id", "parent_id", "name", "gender", "date_of_birth", "created_at", "updated_at", "is_deleted",
	}).AddRow(childID, userID.String(), "Bob", "male", now, now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(parentRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "children" WHERE "children"."parent_id" = $1`)).
		WithArgs(userID.String()).
		WillReturnRows(childRows)

	result, _, _, err := svc.GetUserByID(userID.String())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Alice", result.Name)
	assert.Len(t, result.Children, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetUserByID_ActiveSubscription_ReturnsPremium(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	userID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	parentRows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(userID, "Alice", "081", "alice@example.com", "hash",
		"Jl. Test", "Jakarta", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(parentRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "children" WHERE "children"."parent_id" = $1`)).
		WithArgs(userID.String()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "name", "gender", "date_of_birth", "created_at", "updated_at", "is_deleted"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "status", "expires_at", "provider_order_id", "package_id", "start_date", "auto_renew", "created_at", "updated_at",
		}).AddRow(uuid.New(), userID, "premium", expiresAt, "", nil, now, false, now, now))

	_, status, _, err := svc.GetUserByID(userID.String())

	require.NoError(t, err)
	assert.Equal(t, "premium", status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetUserByID_ActiveSubscription_IncludesPlanNameAndDaysLeft(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(73 * time.Hour) // just over 3 days away

	parentRows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(userID, "Alice", "081", "alice@example.com", "hash",
		"Jl. Test", "Jakarta", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(parentRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "children" WHERE "children"."parent_id" = $1`)).
		WithArgs(userID.String()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "name", "gender", "date_of_birth", "created_at", "updated_at", "is_deleted"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "status", "expires_at", "provider_order_id", "package_id", "start_date", "auto_renew", "created_at", "updated_at",
		}).AddRow(uuid.New(), userID, "premium", expiresAt, "", packageID, now, false, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "name" FROM "premium_packages" WHERE id = $1 ORDER BY "premium_packages"."id" LIMIT $2`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Bulanan"))

	_, status, detail, err := svc.GetUserByID(userID.String())

	require.NoError(t, err)
	assert.Equal(t, "premium", status)
	require.NotNil(t, detail)
	assert.Equal(t, "Bulanan", detail.PlanName)
	require.NotNil(t, detail.DaysLeft)
	assert.Equal(t, 4, *detail.DaysLeft) // ceil(~73h / 24h)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetUserByID_ActiveSubscription_NoPackageID_FallsBackToGenericName(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	userID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	parentRows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(userID, "Alice", "081", "alice@example.com", "hash",
		"Jl. Test", "Jakarta", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(parentRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "children" WHERE "children"."parent_id" = $1`)).
		WithArgs(userID.String()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "name", "gender", "date_of_birth", "created_at", "updated_at", "is_deleted"}))

	// package_id NULL — e.g. an admin manual grant, per models/user_subscription.go's comment.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "status", "expires_at", "provider_order_id", "package_id", "start_date", "auto_renew", "created_at", "updated_at",
		}).AddRow(uuid.New(), userID, "premium", expiresAt, "", nil, now, false, now, now))

	_, status, detail, err := svc.GetUserByID(userID.String())

	require.NoError(t, err)
	assert.Equal(t, "premium", status)
	require.NotNil(t, detail)
	assert.Equal(t, "Langganan Premium", detail.PlanName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetUserByID_ExpiredSubscription_ReturnsFree(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	userID := uuid.New()
	now := time.Now()
	expiredAt := now.Add(-24 * time.Hour)

	parentRows := sqlmock.NewRows([]string{
		"id", "name", "phone_number", "email_address", "password",
		"address", "city", "created_at", "updated_at", "is_deleted",
	}).AddRow(userID, "Alice", "081", "alice@example.com", "hash",
		"Jl. Test", "Jakarta", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(parentRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "children" WHERE "children"."parent_id" = $1`)).
		WithArgs(userID.String()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "name", "gender", "date_of_birth", "created_at", "updated_at", "is_deleted"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "status", "expires_at", "provider_order_id", "package_id", "start_date", "auto_renew", "created_at", "updated_at",
		}).AddRow(uuid.New(), userID, "premium", expiredAt, "", nil, now, false, now, now))

	_, status, _, err := svc.GetUserByID(userID.String())

	require.NoError(t, err)
	assert.Equal(t, "free", status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetUserByID_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("missing-id", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	result, _, _, err := svc.GetUserByID("missing-id")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserService_GetUserByID_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewUserService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "parents" WHERE id = $1 ORDER BY "parents"."id" LIMIT $2`)).
		WithArgs("any-id", 1).
		WillReturnError(sql.ErrConnDone)

	result, _, _, err := svc.GetUserByID("any-id")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}
