package services

import (
	"arunika_backend/models"
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func subWithProviderCols() []string {
	return []string{"id", "user_id", "status", "expires_at", "provider_order_id", "package_id", "start_date", "auto_renew", "provider", "created_at", "updated_at"}
}

// timeNear matches a time argument within a second of want.
type timeNear struct{ want time.Time }

func (m timeNear) Match(v driver.Value) bool {
	t, ok := v.(time.Time)
	if !ok {
		return false
	}
	d := t.Sub(m.want)
	return d < time.Second && d > -time.Second
}

// ─── RenewalFor ──────────────────────────────────────────────────────────────

func TestRenewalFor_WindowBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	sub := func(expires time.Time, autoRenew bool) *models.UserSubscription {
		return &models.UserSubscription{Status: "premium", ExpiresAt: &expires, AutoRenew: autoRenew}
	}

	r := RenewalFor(sub(now.AddDate(0, 0, 8), false), now)
	assert.True(t, r.Active)
	assert.False(t, r.CanRenew, "8 days before expiry is outside the window")

	r = RenewalFor(sub(now.AddDate(0, 0, 7), false), now)
	assert.True(t, r.CanRenew, "exactly 7 days before expiry opens the window")
	assert.Equal(t, now, *r.RenewableFrom)

	r = RenewalFor(sub(now.AddDate(0, 0, 1), false), now)
	assert.True(t, r.CanRenew, "last day")

	r = RenewalFor(sub(now.AddDate(0, 0, 3), true), now)
	assert.True(t, r.Active)
	assert.False(t, r.CanRenew, "auto-renewing subscriptions are never renewed in-app")

	r = RenewalFor(sub(now.Add(-time.Hour), false), now)
	assert.False(t, r.Active, "expired")

	r = RenewalFor(&models.UserSubscription{Status: "premium"}, now)
	assert.True(t, r.Active)
	assert.False(t, r.CanRenew, "no expiry (manual grant) is never renewable")

	assert.False(t, RenewalFor(nil, now).Active)
}

// ─── CheckPurchaseAllowed ────────────────────────────────────────────────────

func expectSubscription(mock sqlmock.Sqlmock, userID uuid.UUID, expires time.Time, autoRenew bool, provider string) {
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows(subWithProviderCols()).
			AddRow(uuid.New(), userID, "premium", expires, "", nil, now, autoRenew, provider, now, now))
}

func TestCheckPurchaseAllowed(t *testing.T) {
	subscriptionPkg := &models.PremiumPackage{Type: "subscription"}
	contentPkg := &models.PremiumPackage{Type: "content"}
	inWindow := time.Now().AddDate(0, 0, 5)
	tooEarly := time.Now().AddDate(0, 0, 20)
	expired := time.Now().AddDate(0, 0, -1)

	cases := []struct {
		name      string
		expires   time.Time
		autoRenew bool
		provider  string
		pkg       *models.PremiumPackage
		viaPlay   bool
		wantErr   bool
	}{
		{"product while subscribed", inWindow, false, "midtrans", nil, false, true},
		{"renewal too early", tooEarly, false, "midtrans", subscriptionPkg, false, true},
		{"renewal inside window", inWindow, false, "midtrans", subscriptionPkg, false, false},
		{"content package inside window", inWindow, false, "midtrans", contentPkg, false, true},
		{"play-managed subscription inside window", inWindow, false, "google_play", subscriptionPkg, false, true},
		{"renewal via a Play order", inWindow, false, "midtrans", subscriptionPkg, true, true},
		{"auto-renewing subscription", inWindow, true, "midtrans", subscriptionPkg, false, true},
		{"expired subscription can buy", expired, false, "midtrans", nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gormDB, mock := setupMockDB(t)
			userID := uuid.New()
			expectSubscription(mock, userID, tc.expires, tc.autoRenew, tc.provider)

			err := NewEntitlementService(gormDB).CheckPurchaseAllowed(userID, tc.pkg, tc.viaPlay)
			if tc.wantErr {
				assert.ErrorIs(t, err, ErrSubscriptionActive)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCheckPurchaseAllowed_NoSubscription(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	userID := uuid.New()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1`)).
		WillReturnRows(sqlmock.NewRows(subWithProviderCols()))

	assert.NoError(t, NewEntitlementService(gormDB).CheckPurchaseAllowed(userID, nil, false))
}

// ─── Stacking ────────────────────────────────────────────────────────────────

// A renewal inside the window adds the new period to the previous expiry,
// switches package_id to the renewed plan and records the order's provider.
func TestGrantForPaidOrder_RenewalStacksFromPreviousExpiry(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)

	userID := uuid.New()
	oldPackageID := uuid.New()
	newPackageID := uuid.New()
	now := time.Now()
	previousExpiry := now.AddDate(0, 0, 5)
	order := &models.Order{ID: uuid.New(), UserID: userID, PackageID: &newPackageID, Provider: models.OrderProviderMidtrans}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "price_idr", "duration_days"}).
			AddRow(newPackageID, "Tahunan", "subscription", 299000, 365))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1`)).
		WillReturnRows(sqlmock.NewRows(subWithProviderCols()).
			AddRow(uuid.New(), userID, "premium", previousExpiry, "", oldPackageID, now, false, "midtrans", now, now))

	mock.ExpectBegin()
	// Map keys are written in sorted order: expires_at, package_id,
	// provider, start_date, status, updated_at, then the WHERE id.
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "user_subscriptions" SET "expires_at"=$1,"package_id"=$2,"provider"=$3`)).
		WithArgs(timeNear{previousExpiry.AddDate(0, 0, 365)}, newPackageID, models.OrderProviderMidtrans,
			sqlmock.AnyArg(), "premium", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, svc.GrantForPaidOrder(gormDB, order))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── Play auto-renew tracking ────────────────────────────────────────────────

func TestHandlePlayRTDN_Canceled_TurnsOffAutoRenew(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: &fakePlayVerifier{}}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND payment_type = $2`)).
		WithArgs("tok-sub", "google_play", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()).
			AddRow(uuid.New(), orderID, "order-"+orderID.String(), &userID, "tok-sub", "settlement", "google_play", "39000.00", "", "", "{}", now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 39000, models.OrderStatusPaid, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "user_subscriptions" SET "auto_renew"=$1,"updated_at"=$2 WHERE user_id = $3`)).
		WithArgs(false, sqlmock.AnyArg(), userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, svc.HandlePlayRTDN(context.Background(), rtdnPayload(playNotifSubscriptionCanceled, "tok-sub", "monthly_premium")))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncSubscriptionExpiry_RecordsPlayProviderAndAutoRenew(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewEntitlementService(gormDB)
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()
	expiry := now.AddDate(0, 1, 0)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1`)).
		WillReturnRows(sqlmock.NewRows(subWithProviderCols()).
			AddRow(uuid.New(), userID, "premium", now, "", packageID, now, false, "midtrans", now, now))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "user_subscriptions" SET "auto_renew"=$1,"expires_at"=$2,"package_id"=$3,"provider"=$4`)).
		WithArgs(true, timeNear{expiry}, packageID, models.OrderProviderGooglePlay, "premium", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, svc.SyncSubscriptionExpiry(gormDB, userID, packageID, expiry, true))
	assert.NoError(t, mock.ExpectationsWereMet())
}
