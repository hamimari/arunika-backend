package services

import (
	"regexp"
	"testing"
	"time"

	"arunika_backend/models"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ─── GrantForPaidOrder: content package fan-out ────────────────────────────────

func TestEntitlementService_GrantForPaidOrder_ContentPackage_FanOut(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	product1 := uuid.New()
	product2 := uuid.New()
	now := time.Now()

	order := &models.Order{ID: orderID, UserID: userID, PackageID: &packageID}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1 ORDER BY "premium_packages"."id" LIMIT $2`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(premiumPackColumns()).
			AddRow(packageID, "Paket Hutan", "8 hewan", 29000, "content", nil, false, true, 1, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_package_items" WHERE package_id = $1`)).
		WithArgs(packageID).
		WillReturnRows(sqlmock.NewRows([]string{"package_id", "product_id", "created_at"}).
			AddRow(packageID, product1, now).
			AddRow(packageID, product2, now))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_entitlements"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_entitlements"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	err := svc.GrantForPaidOrder(gormDB, order)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEntitlementService_GrantForPaidOrder_ContentPackage_IdempotentReGrant(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	productID := uuid.New()
	now := time.Now()

	order := &models.Order{ID: orderID, UserID: userID, PackageID: &packageID}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1 ORDER BY "premium_packages"."id" LIMIT $2`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(premiumPackColumns()).
			AddRow(packageID, "Paket Hutan", "8 hewan", 29000, "content", nil, false, true, 1, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_package_items" WHERE package_id = $1`)).
		WithArgs(packageID).
		WillReturnRows(sqlmock.NewRows([]string{"package_id", "product_id", "created_at"}).
			AddRow(packageID, productID, now))

	// ON CONFLICT DO NOTHING — no row returned, but still not an error.
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_entitlements"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()

	err := svc.GrantForPaidOrder(gormDB, order)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GrantForPaidOrder: subscription package ───────────────────────────────────

func TestEntitlementService_GrantForPaidOrder_SubscriptionPackage_NewSubscription(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()
	duration := 30

	order := &models.Order{ID: orderID, UserID: userID, PackageID: &packageID}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1 ORDER BY "premium_packages"."id" LIMIT $2`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "subtitle", "price_idr", "type",
			"badge_label", "is_best_value", "is_active", "sort_order", "duration_days",
			"created_at", "updated_at",
		}).AddRow(packageID, "Bulanan", "1 bulan", 39000, "subscription", nil, false, true, 1, duration, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_subscriptions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	err := svc.GrantForPaidOrder(gormDB, order)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GrantForPaidOrder: single product (future non-bundle purchase) ────────────

func TestEntitlementService_GrantForPaidOrder_SingleProduct(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	orderID := uuid.New()
	userID := uuid.New()
	productID := uuid.New()

	order := &models.Order{ID: orderID, UserID: userID, ProductID: &productID}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_entitlements"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	err := svc.GrantForPaidOrder(gormDB, order)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEntitlementService_GrantForPaidOrder_SubscriptionPackage_ExtendsActiveSubscription(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()
	duration := 30

	order := &models.Order{ID: orderID, UserID: userID, PackageID: &packageID}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1 ORDER BY "premium_packages"."id" LIMIT $2`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "subtitle", "price_idr", "type",
			"badge_label", "is_best_value", "is_active", "sort_order", "duration_days",
			"created_at", "updated_at",
		}).AddRow(packageID, "Bulanan", "1 bulan", 39000, "subscription", nil, false, true, 1, duration, now, now))

	// Already has an active subscription with 10 days left.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "status", "expires_at", "provider_order_id", "package_id", "start_date", "auto_renew", "created_at", "updated_at",
		}).AddRow(uuid.New(), userID, "premium", now.Add(10*24*time.Hour), "", packageID, now, false, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "user_subscriptions" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.GrantForPaidOrder(gormDB, order)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── computeSubscriptionExpiry ────────────────────────────────────────────────

func TestComputeSubscriptionExpiry_NoExistingSubscription_ExtendsFromNow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	got := computeSubscriptionExpiry(now, nil, 30)

	assert.Equal(t, now.AddDate(0, 0, 30), got)
}

func TestComputeSubscriptionExpiry_ActiveSubscription_ExtendsFromCurrentExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	currentExpiry := now.Add(10 * 24 * time.Hour) // still 10 days left

	got := computeSubscriptionExpiry(now, &currentExpiry, 30)

	// Extends from the existing expiry, not from now — renewing early
	// doesn't lose the remaining 10 days.
	assert.Equal(t, currentExpiry.AddDate(0, 0, 30), got)
}

func TestComputeSubscriptionExpiry_LapsedSubscription_ExtendsFromNow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	currentExpiry := now.Add(-5 * 24 * time.Hour) // expired 5 days ago

	got := computeSubscriptionExpiry(now, &currentExpiry, 365)

	// A long-lapsed subscription has nothing to preserve — extend from now,
	// not from the stale past expiry.
	assert.Equal(t, now.AddDate(0, 0, 365), got)
}

func TestComputeSubscriptionExpiry_YearlyDuration(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	got := computeSubscriptionExpiry(now, nil, 365)

	assert.Equal(t, now.AddDate(0, 0, 365), got)
}

// ─── HasAccess ──────────────────────────────────────────────────────────────────

func TestEntitlementService_HasAccess_ActiveSubscription(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	userID := uuid.New()
	productID := uuid.New()
	future := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "status", "expires_at", "provider_order_id",
			"package_id", "start_date", "auto_renew", "created_at", "updated_at",
		}).AddRow(uuid.New(), userID, "premium", future, "", uuid.New(), time.Now(), false, time.Now(), time.Now()))

	hasAccess, err := svc.HasAccess(userID, productID)

	require.NoError(t, err)
	assert.True(t, hasAccess)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEntitlementService_HasAccess_EntitledNoSubscription(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	userID := uuid.New()
	productID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "user_entitlements" WHERE user_id = $1 AND product_id = $2 AND (expires_at IS NULL OR expires_at > NOW())`)).
		WithArgs(userID, productID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	hasAccess, err := svc.HasAccess(userID, productID)

	require.NoError(t, err)
	assert.True(t, hasAccess)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEntitlementService_HasAccess_NoneNoSubscriptionNoEntitlement(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	userID := uuid.New()
	productID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "user_entitlements" WHERE user_id = $1 AND product_id = $2 AND (expires_at IS NULL OR expires_at > NOW())`)).
		WithArgs(userID, productID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	hasAccess, err := svc.HasAccess(userID, productID)

	require.NoError(t, err)
	assert.False(t, hasAccess)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEntitlementService_HasAccess_ExpiredSubscriptionFallsThrough(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	userID := uuid.New()
	productID := uuid.New()
	past := time.Now().Add(-24 * time.Hour)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "status", "expires_at", "provider_order_id",
			"package_id", "start_date", "auto_renew", "created_at", "updated_at",
		}).AddRow(uuid.New(), userID, "premium", past, "", uuid.New(), time.Now(), false, time.Now(), time.Now()))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "user_entitlements" WHERE user_id = $1 AND product_id = $2 AND (expires_at IS NULL OR expires_at > NOW())`)).
		WithArgs(userID, productID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	hasAccess, err := svc.HasAccess(userID, productID)

	require.NoError(t, err)
	assert.False(t, hasAccess)
	assert.NoError(t, mock.ExpectationsWereMet())
}
