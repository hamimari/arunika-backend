package services

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"arunika_backend/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePlayVerifier is a test double for playPurchaseVerifier — avoids any
// real Google OAuth/HTTP call.
type fakePlayVerifier struct {
	product    *PlayProductPurchase
	productErr error
	sub        *PlaySubscriptionPurchase
	subErr     error

	reportErr    error
	reportedID   string
	reportedReq  CreateExternalTransactionRequest
	reportCalled bool

	voided    []VoidedPurchase
	voidedErr error
}

func (f *fakePlayVerifier) VerifyProductPurchase(ctx context.Context, productID, purchaseToken string) (*PlayProductPurchase, error) {
	return f.product, f.productErr
}

func (f *fakePlayVerifier) ReportExternalTransaction(ctx context.Context, externalTransactionID string, req CreateExternalTransactionRequest) error {
	f.reportCalled = true
	f.reportedID = externalTransactionID
	f.reportedReq = req
	return f.reportErr
}

func (f *fakePlayVerifier) VerifySubscriptionPurchase(ctx context.Context, subscriptionID, purchaseToken string) (*PlaySubscriptionPurchase, error) {
	return f.sub, f.subErr
}

func (f *fakePlayVerifier) ListVoidedPurchases(ctx context.Context, since time.Time) ([]VoidedPurchase, error) {
	return f.voided, f.voidedErr
}

func userSubscriptionCols() []string {
	return []string{
		"id", "user_id", "status", "expires_at", "provider_order_id",
		"package_id", "start_date", "auto_renew", "created_at", "updated_at",
	}
}

// ─── CreatePlayOrder ────────────────────────────────────────────────────────

func TestPaymentService_CreatePlayOrder_NotMapped(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: &fakePlayVerifier{}}

	user := &models.Parent{}
	user.ID = uuid.New()
	pack := &models.PremiumPackage{ID: uuid.New().String(), Type: "content", PriceIdr: 29000}

	order, err := svc.CreatePlayOrder(user, pack)

	assert.ErrorIs(t, err, ErrPackageNotPlayMapped)
	assert.Nil(t, order)
}

func TestPaymentService_CreatePlayOrder_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: &fakePlayVerifier{}}

	playProductID := "pack_bundle_1"
	user := &models.Parent{}
	user.ID = uuid.New()
	pack := &models.PremiumPackage{ID: uuid.New().String(), Type: "content", PriceIdr: 29000, PlayProductID: &playProductID}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "orders"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	order, err := svc.CreatePlayOrder(user, pack)

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, models.OrderStatusPending, order.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── ResolvePendingPlayOrder ────────────────────────────────────────────────

func TestPaymentService_ResolvePendingPlayOrder_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: &fakePlayVerifier{}}

	userID := uuid.New()
	packageID := uuid.New()
	orderID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE play_product_id = $1`)).
		WithArgs("pack_bundle_1", 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now, "pack_bundle_1"))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE user_id = $1 AND package_id = $2 AND status = $3`)).
		WithArgs(userID, packageID, models.OrderStatusPending, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPending, now, now))

	resolved, err := svc.ResolvePendingPlayOrder(userID, "pack_bundle_1")

	require.NoError(t, err)
	assert.Equal(t, orderID, resolved)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_ResolvePendingPlayOrder_NoPendingOrder(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: &fakePlayVerifier{}}

	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE play_product_id = $1`)).
		WithArgs("pack_bundle_1", 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now, "pack_bundle_1"))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE user_id = $1 AND package_id = $2 AND status = $3`)).
		WithArgs(userID, packageID, models.OrderStatusPending, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()))

	_, err := svc.ResolvePendingPlayOrder(userID, "pack_bundle_1")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_ResolvePendingPlayOrder_UnmappedProduct(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: &fakePlayVerifier{}}

	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE play_product_id = $1`)).
		WithArgs("unknown_product", 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")))

	_, err := svc.ResolvePendingPlayOrder(userID, "unknown_product")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── VerifyPlayPurchase ─────────────────────────────────────────────────────

func TestPaymentService_VerifyPlayPurchase_ContentSuccess(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{product: &PlayProductPurchase{PurchaseState: PlayPurchaseStatePurchased, OrderID: "GPA.1"}}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	productID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 AND user_id = $2 ORDER BY "orders"."id" LIMIT $3 FOR UPDATE`)).
		WithArgs(orderID, userID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPending, now, now))

	// reused-token check — no existing payment for this token on another order
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND order_id != $2`)).
		WithArgs("tok-abc", orderID, 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now, "pack_bundle_1"))

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// EntitlementService.GrantForPaidOrder re-loads the package itself.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now, "pack_bundle_1"))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_package_items" WHERE package_id = $1`)).
		WithArgs(packageID).
		WillReturnRows(sqlmock.NewRows([]string{"package_id", "product_id", "created_at"}).
			AddRow(packageID, productID, now))

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_entitlements"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	mock.ExpectCommit()

	order, err := svc.VerifyPlayPurchase(context.Background(), orderID, userID, "tok-abc")

	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusPaid, order.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Subscription purchases write user_subscriptions twice: upsertSubscription
// inserts it from the package's duration_days, then SyncSubscriptionExpiry
// overwrites the expiry with Play's own.
//
// Note this cannot catch the deadlock those two writes caused when the second
// ran off s.db instead of the transaction — sqlmock serves both from one
// connection, so it never reproduces the lock wait on the UNIQUE user_id.
// Only the real database does.
func TestPaymentService_VerifyPlayPurchase_SubscriptionSuccess(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	playExpiry := time.Now().Add(30 * 24 * time.Hour)
	fake := &fakePlayVerifier{sub: &PlaySubscriptionPurchase{
		OrderID:          "GPA.2",
		ExpiryTimeMillis: fmt.Sprintf("%d", playExpiry.UnixMilli()),
	}}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	subID := uuid.New()
	now := time.Now()

	subPackRow := func() *sqlmock.Rows {
		return sqlmock.NewRows(append(premiumPackColumns(), "duration_days", "play_product_id")).
			AddRow(packageID, "Bulanan", "1 bulan", nil, nil, 39000, "subscription", nil, false, true, 1, now, now, 30, "arunika_monthly")
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 AND user_id = $2 ORDER BY "orders"."id" LIMIT $3 FOR UPDATE`)).
		WithArgs(orderID, userID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 39000, models.OrderStatusPending, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND order_id != $2`)).
		WithArgs("tok-sub", orderID, 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(subPackRow())

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// GrantForPaidOrder re-loads the package, then upserts the subscription.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(subPackRow())

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows(userSubscriptionCols()))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_subscriptions"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(subID))

	// SyncSubscriptionExpiry, on the same transaction, now sees that insert
	// and updates it to Play's expiry rather than inserting a second row.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows(userSubscriptionCols()).
			AddRow(subID, userID, "premium", now, "", packageID, now, false, now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "user_subscriptions" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	order, err := svc.VerifyPlayPurchase(context.Background(), orderID, userID, "tok-sub")

	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusPaid, order.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_VerifyPlayPurchase_NotValid(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{product: &PlayProductPurchase{PurchaseState: PlayPurchaseStateCanceled}}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 AND user_id = $2 ORDER BY "orders"."id" LIMIT $3 FOR UPDATE`)).
		WithArgs(orderID, userID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPending, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND order_id != $2`)).
		WithArgs("tok-bad", orderID, 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now, "pack_bundle_1"))

	mock.ExpectRollback()

	order, err := svc.VerifyPlayPurchase(context.Background(), orderID, userID, "tok-bad")

	assert.ErrorIs(t, err, ErrPlayPurchaseNotValid)
	assert.Nil(t, order)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_VerifyPlayPurchase_TokenReused(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: &fakePlayVerifier{}}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	otherOrderID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 AND user_id = $2 ORDER BY "orders"."id" LIMIT $3 FOR UPDATE`)).
		WithArgs(orderID, userID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPending, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND order_id != $2`)).
		WithArgs("tok-shared", orderID, 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()).
			AddRow(uuid.New(), otherOrderID, "order-"+otherOrderID.String(), &userID, "tok-shared", "settlement", "google_play", "29000.00", "", "", "{}", now, now))

	mock.ExpectRollback()

	order, err := svc.VerifyPlayPurchase(context.Background(), orderID, userID, "tok-shared")

	assert.ErrorIs(t, err, ErrPlayPurchaseTokenReused)
	assert.Nil(t, order)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_VerifyPlayPurchase_IdempotentReplay(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: &fakePlayVerifier{}}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 AND user_id = $2 ORDER BY "orders"."id" LIMIT $3 FOR UPDATE`)).
		WithArgs(orderID, userID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPaid, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE order_id = $1 AND transaction_id = $2`)).
		WithArgs(orderID, "tok-abc", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()).
			AddRow(uuid.New(), orderID, "order-"+orderID.String(), &userID, "tok-abc", "settlement", "google_play", "29000.00", "", "", "{}", now, now))

	mock.ExpectCommit()

	order, err := svc.VerifyPlayPurchase(context.Background(), orderID, userID, "tok-abc")

	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusPaid, order.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── ReconcileVoidedPurchases ───────────────────────────────────────────────

func TestPaymentService_ReconcileVoidedPurchases_RevokesContentEntitlement(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{voided: []VoidedPurchase{{PurchaseToken: "tok-voided", PurchaseSpecifiedType: 1}}}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND payment_type = $2`)).
		WithArgs("tok-voided", "google_play", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()).
			AddRow(uuid.New(), orderID, "order-"+orderID.String(), &userID, "tok-voided", "settlement", "google_play", "29000.00", "", "", "{}", now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPaid, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now, "pack_bundle_1"))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "user_entitlements" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	count, err := svc.ReconcileVoidedPurchases(context.Background(), now.Add(-24*time.Hour))

	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_ReconcileVoidedPurchases_RevokesSubscription(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{voided: []VoidedPurchase{{PurchaseToken: "tok-sub-voided", PurchaseSpecifiedType: 0}}}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND payment_type = $2`)).
		WithArgs("tok-sub-voided", "google_play", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()).
			AddRow(uuid.New(), orderID, "order-"+orderID.String(), &userID, "tok-sub-voided", "settlement", "google_play", "39000.00", "", "", "{}", now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 39000, models.OrderStatusPaid, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Bulanan", "1 bulan", nil, nil, 39000, "subscription", nil, false, true, 1, now, now, "monthly_premium"))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "user_subscriptions" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	count, err := svc.ReconcileVoidedPurchases(context.Background(), now.Add(-24*time.Hour))

	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_ReconcileVoidedPurchases_UnknownToken_NoOp(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{voided: []VoidedPurchase{{PurchaseToken: "tok-unknown", PurchaseSpecifiedType: 1}}}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND payment_type = $2`)).
		WithArgs("tok-unknown", "google_play", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()))

	count, err := svc.ReconcileVoidedPurchases(context.Background(), time.Now().Add(-24*time.Hour))

	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_ReconcileVoidedPurchases_AlreadyReconciled_NoOp(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{voided: []VoidedPurchase{{PurchaseToken: "tok-abc", PurchaseSpecifiedType: 1}}}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND payment_type = $2`)).
		WithArgs("tok-abc", "google_play", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()).
			AddRow(uuid.New(), orderID, "order-"+orderID.String(), &userID, "tok-abc", "settlement", "google_play", "29000.00", "", "", "{}", now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusRefunded, now, now))

	count, err := svc.ReconcileVoidedPurchases(context.Background(), now.Add(-24*time.Hour))

	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── HandlePlayRTDN ─────────────────────────────────────────────────────────

func rtdnPayload(notificationType int, purchaseToken, subscriptionID string) []byte {
	return []byte(fmt.Sprintf(
		`{"packageName":"com.arunika","subscriptionNotification":{"notificationType":%d,"purchaseToken":%q,"subscriptionId":%q}}`,
		notificationType, purchaseToken, subscriptionID,
	))
}

func TestPaymentService_HandlePlayRTDN_Renewed_SyncsExpiry(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{sub: &PlaySubscriptionPurchase{OrderID: "GPA.9", ExpiryTimeMillis: "4102444800000"}}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

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

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "status", "expires_at", "provider_order_id", "package_id", "start_date", "auto_renew", "created_at", "updated_at"}).
			AddRow(uuid.New(), userID, "premium", now, "", packageID, now, false, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "user_subscriptions" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.HandlePlayRTDN(context.Background(), rtdnPayload(playNotifSubscriptionRenewed, "tok-sub", "monthly_premium"))

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_HandlePlayRTDN_Revoked_RevokesSubscription(t *testing.T) {
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
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "user_subscriptions" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.HandlePlayRTDN(context.Background(), rtdnPayload(playNotifSubscriptionRevoked, "tok-sub", "monthly_premium"))

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_HandlePlayRTDN_UnknownToken_NoOp(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: &fakePlayVerifier{}}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE transaction_id = $1 AND payment_type = $2`)).
		WithArgs("tok-unknown", "google_play", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()))

	err := svc.HandlePlayRTDN(context.Background(), rtdnPayload(playNotifSubscriptionRenewed, "tok-unknown", "monthly_premium"))

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── ReportExternalTransaction ──────────────────────────────────────────────

func TestPaymentService_ReportExternalTransaction_ContentSuccess(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPaid, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE order_id = $1 AND payment_type = $2`)).
		WithArgs(orderID, "google_play_external_report", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now, "pack_bundle_1"))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	order, err := svc.ReportExternalTransaction(context.Background(), orderID, userID, "ext-token-abc")

	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusPaid, order.Status)
	assert.True(t, fake.reportCalled)
	assert.Equal(t, orderID.String(), fake.reportedID)
	require.NotNil(t, fake.reportedReq.OneTimeTransaction)
	assert.Equal(t, "ext-token-abc", fake.reportedReq.OneTimeTransaction.ExternalTransactionToken)
	assert.Nil(t, fake.reportedReq.RecurringTransaction)
	assert.Equal(t, "29000000000", fake.reportedReq.OriginalPreTaxAmount.PriceMicros)
	assert.Equal(t, "IDR", fake.reportedReq.OriginalPreTaxAmount.Currency)
	assert.Equal(t, "ID", fake.reportedReq.UserTaxAddress.RegionCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_ReportExternalTransaction_SubscriptionSuccess(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 39000, models.OrderStatusPaid, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE order_id = $1 AND payment_type = $2`)).
		WithArgs(orderID, "google_play_external_report", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Bulanan", "1 bulan", nil, nil, 39000, "subscription", nil, false, true, 1, now, now, "monthly_premium"))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	_, err := svc.ReportExternalTransaction(context.Background(), orderID, userID, "ext-token-sub")

	require.NoError(t, err)
	require.NotNil(t, fake.reportedReq.RecurringTransaction)
	assert.Equal(t, "ext-token-sub", fake.reportedReq.RecurringTransaction.ExternalTransactionToken)
	assert.Equal(t, "RECURRING", fake.reportedReq.RecurringTransaction.ExternalSubscription.SubscriptionType)
	assert.Nil(t, fake.reportedReq.OneTimeTransaction)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_ReportExternalTransaction_AlreadyReported_NoOp(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPaid, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE order_id = $1 AND payment_type = $2`)).
		WithArgs(orderID, "google_play_external_report", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()).
			AddRow(uuid.New(), orderID, "extxn-"+orderID.String(), &userID, "old-token", "reported", "google_play_external_report", "29000.00", "", "", "{}", now, now))

	order, err := svc.ReportExternalTransaction(context.Background(), orderID, userID, "ext-token-new")

	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusPaid, order.Status)
	assert.False(t, fake.reportCalled)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_ReportExternalTransaction_OrderNotPaid(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPending, now, now))

	_, err := svc.ReportExternalTransaction(context.Background(), orderID, userID, "ext-token-abc")

	assert.ErrorIs(t, err, ErrOrderNotPaid)
	assert.False(t, fake.reportCalled)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_ReportExternalTransaction_GoogleApiError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	fake := &fakePlayVerifier{reportErr: fmt.Errorf("android publisher error 400: bad request")}
	svc := &PaymentService{db: gormDB, entitlementService: NewEntitlementService(gormDB), playVerifier: fake}

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, models.OrderStatusPaid, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "payments" WHERE order_id = $1 AND payment_type = $2`)).
		WithArgs(orderID, "google_play_external_report", 1).
		WillReturnRows(sqlmock.NewRows(paymentCols()))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(append(premiumPackColumns(), "play_product_id")).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now, "pack_bundle_1"))

	_, err := svc.ReportExternalTransaction(context.Background(), orderID, userID, "ext-token-abc")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
