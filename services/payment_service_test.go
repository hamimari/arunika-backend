package services

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ─── HandleNotification tests ─────────────────────────────────────────────────

func TestPaymentService_HandleNotification_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPaymentService(gormDB)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "payment_transactions" SET`,
	)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	notification := MidtransNotification{
		OrderID:           "order-123",
		TransactionStatus: "settlement",
		FraudStatus:       "accept",
		PaymentType:       "bank_transfer",
		GrossAmount:       "50000",
	}

	err := svc.HandleNotification(notification)

	require.NoError(t, err)
}

func TestPaymentService_HandleNotification_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPaymentService(gormDB)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(
		`UPDATE "payment_transactions" SET`,
	)).WillReturnError(gorm.ErrInvalidDB)
	mock.ExpectRollback()

	notification := MidtransNotification{
		OrderID:           "order-err",
		TransactionStatus: "pending",
	}

	err := svc.HandleNotification(notification)

	assert.Error(t, err)
}

// ─── CreateSnapToken tests ─────────────────────────────────────────────────────

func TestPaymentService_CreateSnapToken_MidtransError(t *testing.T) {
	// Stand up a fake Midtrans server that returns 500.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"server error"}`)
	}))
	defer ts.Close()

	t.Setenv("MIDTRANS_SNAP_URL", ts.URL)
	t.Setenv("MIDTRANS_SERVER_KEY", "fake-key")

	gormDB, _ := setupMockDB(t)
	svc := NewPaymentService(gormDB)

	tx, err := svc.CreateSnapToken("user-1", "Basic Plan", 50000)

	assert.Error(t, err)
	assert.Nil(t, tx)
}

func TestPaymentService_CreateSnapToken_Success(t *testing.T) {
	// Fake Midtrans that returns a valid snap token.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"token":"tok-abc","redirect_url":"https://app.midtrans.com/snap/tok-abc"}`)
	}))
	defer ts.Close()

	t.Setenv("MIDTRANS_SNAP_URL", ts.URL)
	t.Setenv("MIDTRANS_SERVER_KEY", "fake-key")

	gormDB, mock := setupMockDB(t)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(
		`INSERT INTO "payment_transactions"`,
	)).WillReturnRows(sqlmock.NewRows([]string{"id", "paid_at"}).AddRow(uuid.New().String(), nil))
	mock.ExpectCommit()

	svc := NewPaymentService(gormDB)

	result, err := svc.CreateSnapToken("user-1", "Basic Plan", 50000)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "tok-abc", result.SnapToken)
	assert.Equal(t, "pending", result.Status)
	_ = time.Now() // imported for consistency with other test files
}
