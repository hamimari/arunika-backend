package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// order_refunds (V59): the schema itself enforces the audit rules.

func newRefund(order *models.Order, source, status string) *models.OrderRefund {
	reason := "Pengguna salah beli kartu"
	admin := uuid.New()
	r := &models.OrderRefund{
		OrderID: order.ID, Source: source, Status: status,
		OrderAmountIdr: order.AmountIdr, RequestedAt: time.Now(),
	}
	if source == models.RefundSourceAdmin {
		r.Reason, r.AdminID = &reason, &admin
	}
	return r
}

func TestOrderRefunds_AdminRefundNeedsReasonAndAdmin(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	order := fixtures.NewPaidOrder(t, db, fixtures.NewUser(t, db), fixtures.NewProduct(t, db))

	bare := &models.OrderRefund{
		OrderID: order.ID, Source: models.RefundSourceAdmin, Status: models.RefundStatusRequested,
		OrderAmountIdr: order.AmountIdr, RequestedAt: time.Now(),
	}
	require.Error(t, db.Create(bare).Error, "an admin refund without a reason and admin is rejected")
	require.NoError(t, db.Create(newRefund(order, models.RefundSourceGoogleVoided, models.RefundStatusSucceeded)).Error,
		"a Google-initiated refund has no admin or reason")
}

func TestOrderRefunds_OnlyOneActiveRefundPerOrder(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	order := fixtures.NewPaidOrder(t, db, fixtures.NewUser(t, db), fixtures.NewProduct(t, db))

	require.NoError(t, db.Create(newRefund(order, models.RefundSourceAdmin, models.RefundStatusFailed)).Error)
	require.NoError(t, db.Create(newRefund(order, models.RefundSourceAdmin, models.RefundStatusFailed)).Error,
		"failed attempts don't block a retry")
	require.NoError(t, db.Create(newRefund(order, models.RefundSourceAdmin, models.RefundStatusRequested)).Error)
	require.Error(t, db.Create(newRefund(order, models.RefundSourceAdmin, models.RefundStatusRequested)).Error,
		"a second in-progress refund of the same order is rejected")
}

func TestFeatureFlags_AlternativeBillingSeededOff(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	var flag models.FeatureFlag
	require.NoError(t, db.First(&flag, "key = ?", models.FeatureFlagAlternativeBilling).Error)
	require.False(t, flag.IsEnabled, "Midtrans stays off until an admin turns it on")
}
