package services

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"arunika_backend/models"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func computeSignature(orderID, statusCode, grossAmount, serverKey string) string {
	h := sha512.New()
	h.Write([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(h.Sum(nil))
}

// ─── ValidateWebhookSignature ─────────────────────────────────────────────────

func TestValidateWebhookSignature_Valid(t *testing.T) {
	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	sig := computeSignature("ORDER-001", "200", "49000.00", "testkey")
	assert.True(t, ValidateWebhookSignature("ORDER-001", "200", "49000.00", sig))
}

func TestValidateWebhookSignature_Invalid(t *testing.T) {
	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")
	assert.False(t, ValidateWebhookSignature("ORDER-001", "200", "49000.00", "wrongsig"))
}

func TestValidateWebhookSignature_WrongKey(t *testing.T) {
	os.Setenv("MIDTRANS_SERVER_KEY", "correctkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	// Signature computed with a different key — should fail.
	sig := computeSignature("ORDER-001", "200", "49000.00", "wrongkey")
	assert.False(t, ValidateWebhookSignature("ORDER-001", "200", "49000.00", sig))
}

// ─── CreateSnapTransaction ──────────────────────────────────────────────────────

func TestPaymentService_CreateSnapTransaction_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))

	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":        "snap-token-abc",
			"redirect_url": "https://example.com/pay",
		})
	}))
	defer server.Close()
	os.Setenv("MIDTRANS_BASE_URL", server.URL)
	defer os.Unsetenv("MIDTRANS_BASE_URL")

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "orders"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "payments" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	user := &models.Parent{Name: "Jane", EmailAddress: "jane@example.com", PhoneNumber: "0812"}
	user.ID = uuid.New()
	pack := &models.PremiumPackage{ID: uuid.New().String(), Name: "Bulanan", PriceIdr: 39000, Type: "subscription"}

	resp, err := svc.CreateSnapTransaction(user, pack)

	require.NoError(t, err)
	assert.Equal(t, "snap-token-abc", resp.Token)
	assert.Equal(t, "https://example.com/pay", resp.RedirectURL)
	assert.NotEmpty(t, resp.OrderID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_CreateSnapTransaction_NoServerKey(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))
	os.Unsetenv("MIDTRANS_SERVER_KEY")

	user := &models.Parent{Name: "Jane", EmailAddress: "jane@example.com"}
	pack := &models.PremiumPackage{ID: uuid.New().String(), Name: "Bulanan", PriceIdr: 39000, Type: "subscription"}

	resp, err := svc.CreateSnapTransaction(user, pack)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestPaymentService_CreateSnapTransactionForProduct_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))

	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `"name":"Singa"`)
		assert.Contains(t, string(body), `"gross_amount":15000`)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token":        "snap-token-product",
			"redirect_url": "https://example.com/pay-product",
		})
	}))
	defer server.Close()
	os.Setenv("MIDTRANS_BASE_URL", server.URL)
	defer os.Unsetenv("MIDTRANS_BASE_URL")

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "orders"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "payments" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	user := &models.Parent{Name: "Jane", EmailAddress: "jane@example.com", PhoneNumber: "0812"}
	user.ID = uuid.New()
	product := &models.Product{ID: uuid.New(), PriceIdr: 15000}

	resp, err := svc.CreateSnapTransactionForProduct(user, product, "Singa")

	require.NoError(t, err)
	assert.Equal(t, "snap-token-product", resp.Token)
	assert.Equal(t, "https://example.com/pay-product", resp.RedirectURL)
	assert.NotEmpty(t, resp.OrderID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── HandleWebhook ──────────────────────────────────────────────────────────────

func orderCols() []string {
	return []string{"id", "user_id", "product_id", "package_id", "amount_idr", "status", "created_at", "updated_at"}
}

func TestPaymentService_HandleWebhook_InvalidSignature(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))
	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	notif := WebhookNotification{
		OrderID:           "order-" + uuid.New().String(),
		StatusCode:        "200",
		GrossAmount:       "29000.00",
		TransactionStatus: "settlement",
		SignatureKey:      "wrong",
	}

	userID, err := svc.HandleWebhook(notif)
	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, userID)
}

func TestPaymentService_HandleWebhook_UnrecognizedOrderIDFormat(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))
	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	notif := WebhookNotification{
		OrderID:           "sub-legacy-format",
		StatusCode:        "200",
		GrossAmount:       "29000.00",
		TransactionStatus: "settlement",
	}
	notif.SignatureKey = computeSignature(notif.OrderID, notif.StatusCode, notif.GrossAmount, "testkey")

	userID, err := svc.HandleWebhook(notif)
	assert.Error(t, err)
	assert.Equal(t, uuid.Nil, userID)
}

func TestPaymentService_HandleWebhook_HappyPath_ContentPackage(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))

	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	productID := uuid.New()
	now := time.Now()

	notif := WebhookNotification{
		OrderID:           "order-" + orderID.String(),
		StatusCode:        "200",
		GrossAmount:       "29000.00",
		TransactionStatus: "settlement",
		TransactionID:     "txn-1",
		PaymentType:       "gopay",
	}
	notif.SignatureKey = computeSignature(notif.OrderID, notif.StatusCode, notif.GrossAmount, "testkey")

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2 FOR UPDATE`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, "PENDING", now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(premiumPackColumns()).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_package_items" WHERE package_id = $1`)).
		WithArgs(packageID).
		WillReturnRows(sqlmock.NewRows([]string{"package_id", "product_id", "created_at"}).
			AddRow(packageID, productID, now))

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_entitlements"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	mock.ExpectCommit()

	gotUserID, err := svc.HandleWebhook(notif)

	require.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── SyncOrderStatus ──────────────────────────────────────────────────────────

func TestPaymentService_SyncOrderStatus_NotPending_NoOp(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))

	order := &models.Order{ID: uuid.New(), Status: models.OrderStatusPaid}
	got := svc.SyncOrderStatus(order)

	assert.Same(t, order, got)
}

func TestPaymentService_SyncOrderStatus_NoServerKey_NoOp(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))
	os.Unsetenv("MIDTRANS_SERVER_KEY")

	order := &models.Order{ID: uuid.New(), Status: models.OrderStatusPending}
	got := svc.SyncOrderStatus(order)

	assert.Same(t, order, got)
}

func TestPaymentService_SyncOrderStatus_StillPending_NoOp(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))

	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"transaction_status": "pending"})
	}))
	defer server.Close()
	os.Setenv("MIDTRANS_CORE_API_URL", server.URL)
	defer os.Unsetenv("MIDTRANS_CORE_API_URL")

	order := &models.Order{ID: uuid.New(), Status: models.OrderStatusPending}
	got := svc.SyncOrderStatus(order)

	assert.Equal(t, models.OrderStatusPending, got.Status)
}

func TestPaymentService_SyncOrderStatus_MidtransUnreachable_FailsOpen(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))

	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")
	os.Setenv("MIDTRANS_CORE_API_URL", "http://127.0.0.1:1")
	defer os.Unsetenv("MIDTRANS_CORE_API_URL")

	order := &models.Order{ID: uuid.New(), Status: models.OrderStatusPending}
	got := svc.SyncOrderStatus(order)

	assert.Equal(t, models.OrderStatusPending, got.Status)
}

func TestPaymentService_SyncOrderStatus_Settled_TransitionsOrder(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))

	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	productID := uuid.New()
	now := time.Now()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/order-"+orderID.String()+"/status", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"transaction_status": "settlement",
			"fraud_status":       "accept",
			"status_code":        "200",
			"gross_amount":       "29000.00",
			"transaction_id":     "txn-sync-1",
			"payment_type":       "gopay",
		})
	}))
	defer server.Close()
	os.Setenv("MIDTRANS_CORE_API_URL", server.URL)
	defer os.Unsetenv("MIDTRANS_CORE_API_URL")

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2 FOR UPDATE`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, "PENDING", now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "orders" SET`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_packages" WHERE id = $1`)).
		WithArgs(packageID.String(), 1).
		WillReturnRows(sqlmock.NewRows(premiumPackColumns()).
			AddRow(packageID, "Paket Hutan", "8 hewan", nil, nil, 29000, "content", nil, false, true, 1, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_package_items" WHERE package_id = $1`)).
		WithArgs(packageID).
		WillReturnRows(sqlmock.NewRows([]string{"package_id", "product_id", "created_at"}).
			AddRow(packageID, productID, now))

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "user_entitlements"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	mock.ExpectCommit()

	order := &models.Order{ID: orderID, UserID: userID, Status: models.OrderStatusPending}
	got := svc.SyncOrderStatus(order)

	assert.Equal(t, models.OrderStatusPaid, got.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentService_HandleWebhook_IdempotentReplay(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPaymentService(gormDB, NewEntitlementService(gormDB))

	os.Setenv("MIDTRANS_SERVER_KEY", "testkey")
	defer os.Unsetenv("MIDTRANS_SERVER_KEY")

	orderID := uuid.New()
	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	notif := WebhookNotification{
		OrderID:           "order-" + orderID.String(),
		StatusCode:        "200",
		GrossAmount:       "29000.00",
		TransactionStatus: "settlement",
		TransactionID:     "txn-1",
		PaymentType:       "gopay",
	}
	notif.SignatureKey = computeSignature(notif.OrderID, notif.StatusCode, notif.GrossAmount, "testkey")

	// Order is already PAID (from the first delivery) — replay must not
	// re-flip status or re-grant entitlements, only record the callback.
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "orders" WHERE id = $1 ORDER BY "orders"."id" LIMIT $2 FOR UPDATE`)).
		WithArgs(orderID, 1).
		WillReturnRows(sqlmock.NewRows(orderCols()).
			AddRow(orderID, userID, nil, packageID, 29000, "PAID", now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "payments"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))

	mock.ExpectCommit()

	gotUserID, err := svc.HandleWebhook(notif)

	require.NoError(t, err)
	assert.Equal(t, userID, gotUserID)
	assert.NoError(t, mock.ExpectationsWereMet())
}
