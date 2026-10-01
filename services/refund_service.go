package services

import (
	"arunika_backend/models"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Refunds of Google Play orders — issued by an admin from the backoffice, or
// reported by Google itself — and the order_refunds records kept for each.

const (
	// minRefundReasonLength keeps the audit trail useful: "refund" alone
	// explains nothing to whoever reads it later.
	minRefundReasonLength = 10
	// playRefundWindow is how far back Google accepts orders.refund.
	playRefundWindowYears = 3
)

var (
	// ErrOrderNotRefundable: not a PAID Google Play order with what Google
	// needs to refund it (or older than Google's refund window).
	ErrOrderNotRefundable = errors.New("order cannot be refunded")
	// ErrRefundInProgress: the order already has a requested or completed
	// refund.
	ErrRefundInProgress = errors.New("order already has a refund in progress or completed")
)

// PlayRefundError wraps Google's refusal of a refund, so handlers can report
// it as an upstream failure with Google's own message.
type PlayRefundError struct{ Err error }

func (e *PlayRefundError) Error() string { return "google play refused the refund: " + e.Err.Error() }
func (e *PlayRefundError) Unwrap() error { return e.Err }

// RefundInput is an admin's refund request.
type RefundInput struct {
	Reason     string `json:"reason"`
	RefundType string `json:"refund_type"` // FULL (default) | PRORATED (subscriptions only)
}

// RefundPlayOrder refunds a PAID Google Play order through the Android
// Publisher API and, on success, marks it REFUNDED and removes the access it
// granted. One-time purchases use orders.refund (revoke=true); subscriptions
// use subscriptionsv2.revoke with a full or prorated refund. Every attempt is
// recorded in order_refunds — a Google failure as FAILED, leaving the order
// PAID.
func (s *PaymentService) RefundPlayOrder(ctx context.Context, orderID, adminID uuid.UUID, in RefundInput) (*models.OrderRefund, error) {
	reason := strings.TrimSpace(in.Reason)
	if len([]rune(reason)) < minRefundReasonLength {
		return nil, validationErrorf("reason must be at least %d characters", minRefundReasonLength)
	}
	refundType := in.RefundType
	if refundType == "" {
		refundType = models.RefundTypeFull
	}
	if refundType != models.RefundTypeFull && refundType != models.RefundTypeProrated {
		return nil, validationErrorf("refund_type must be FULL or PRORATED")
	}

	order, err := models.FindOrderByID(s.db, orderID)
	if err != nil {
		return nil, err
	}
	if order.Provider != models.OrderProviderGooglePlay || order.Status != models.OrderStatusPaid ||
		order.PurchaseToken == nil || *order.PurchaseToken == "" ||
		order.CreatedAt.AddDate(playRefundWindowYears, 0, 0).Before(time.Now()) {
		return nil, ErrOrderNotRefundable
	}
	isSub, err := s.isSubscriptionOrder(order)
	if err != nil {
		return nil, err
	}
	if !isSub && refundType == models.RefundTypeProrated {
		return nil, validationErrorf("prorated refunds apply to subscriptions only")
	}
	playOrderID, err := s.latestPlayOrderID(order.ID)
	if err != nil {
		return nil, err
	}
	if !isSub && playOrderID == "" {
		return nil, ErrOrderNotRefundable
	}

	refund := models.OrderRefund{
		OrderID:        order.ID,
		Provider:       models.OrderProviderGooglePlay,
		Source:         models.RefundSourceAdmin,
		RefundType:     refundType,
		Revoked:        true,
		Reason:         &reason,
		AdminID:        &adminID,
		PlayOrderID:    optionalString(playOrderID),
		PurchaseToken:  order.PurchaseToken,
		OrderAmountIdr: order.AmountIdr,
		Status:         models.RefundStatusRequested,
		RequestedAt:    time.Now(),
	}
	if err := s.db.Create(&refund).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrRefundInProgress
		}
		return nil, fmt.Errorf("record refund: %w", err)
	}

	var callErr error
	if isSub {
		callErr = s.playVerifier.RevokeSubscriptionPurchase(ctx, *order.PurchaseToken, refundType == models.RefundTypeProrated)
	} else {
		callErr = s.playVerifier.RefundOrder(ctx, playOrderID, true)
	}
	if callErr != nil && playOrderID != "" {
		// Google refuses a second refund; if it already refunded this order
		// (e.g. from Play Console), finish the job here instead of failing.
		if o, err := s.playVerifier.GetOrder(ctx, playOrderID); err == nil && o.IsRefunded() {
			note := "already refunded at Google: " + callErr.Error()
			refund.Error = &note
			callErr = nil
		}
	}
	if callErr != nil {
		msg := callErr.Error()
		now := time.Now()
		refund.Status, refund.Error, refund.CompletedAt = models.RefundStatusFailed, &msg, &now
		if err := s.db.Model(&refund).Updates(map[string]interface{}{
			"status": refund.Status, "error": msg, "completed_at": now, "updated_at": now,
		}).Error; err != nil {
			slog.Error("RefundPlayOrder: failed to record Google refusal", "refund_id", refund.ID, "error", err)
		}
		return &refund, &PlayRefundError{Err: callErr}
	}

	if err := s.completeRefund(order, &refund); err != nil {
		return &refund, err
	}
	s.fillRefundDetails(ctx, &refund)
	return &refund, nil
}

// completeRefund marks a Google-accepted refund as done locally: the order
// becomes REFUNDED, its access is revoked, and the refund SUCCEEDED. If the
// local update fails the refund stays REQUESTED; SyncRefundDetails (or the
// voided-purchases reconcile) finishes it later.
func (s *PaymentService) completeRefund(order *models.Order, refund *models.OrderRefund) error {
	if err := s.db.Model(order).Where("status = ?", models.OrderStatusPaid).
		Update("status", models.OrderStatusRefunded).Error; err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	if err := s.revokeOrderAccess(order); err != nil {
		return err
	}
	now := time.Now()
	refund.Status, refund.CompletedAt = models.RefundStatusSucceeded, &now
	return s.db.Model(refund).Updates(map[string]interface{}{
		"status": refund.Status, "completed_at": now, "error": refund.Error, "updated_at": now,
	}).Error
}

// revokeOrderAccess removes what a refunded order granted: the whole
// subscription for a subscription package, otherwise that order's
// entitlements.
func (s *PaymentService) revokeOrderAccess(order *models.Order) error {
	isSub, err := s.isSubscriptionOrder(order)
	if err != nil {
		return err
	}
	if isSub {
		if err := s.entitlementService.RevokeSubscription(order.UserID); err != nil {
			return fmt.Errorf("revoke subscription: %w", err)
		}
		return nil
	}
	if err := s.entitlementService.RevokeEntitlementForOrder(order.ID); err != nil {
		return fmt.Errorf("revoke entitlement: %w", err)
	}
	return nil
}

func (s *PaymentService) isSubscriptionOrder(order *models.Order) (bool, error) {
	if order.PackageID == nil {
		return false, nil
	}
	pkg, err := models.FindPremiumPackageByID(s.db, order.PackageID.String())
	if err != nil {
		return false, fmt.Errorf("load package: %w", err)
	}
	return pkg.Type == "subscription", nil
}

// latestPlayOrderID is the Google Play order id (GPA.…) recorded when the
// purchase was verified, or "" if none is on file.
func (s *PaymentService) latestPlayOrderID(orderID uuid.UUID) (string, error) {
	var payment models.Payment
	err := s.db.Where("order_id = ? AND payment_type = ? AND provider_order_id <> ''", orderID, "google_play").
		Order("created_at DESC").First(&payment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return payment.ProviderOrderID, nil
}

// fillRefundDetails records what Google reports for the refunded order —
// amounts, state, reason and the raw response. Best effort: a refund Google
// accepted still counts if this lookup fails.
func (s *PaymentService) fillRefundDetails(ctx context.Context, refund *models.OrderRefund) {
	if refund.PlayOrderID == nil || *refund.PlayOrderID == "" {
		return
	}
	order, err := s.playVerifier.GetOrder(ctx, *refund.PlayOrderID)
	if err != nil {
		slog.Warn("refund details unavailable from Google", "refund_id", refund.ID, "error", err)
		return
	}
	updates := map[string]interface{}{"updated_at": time.Now()}
	state := order.State
	refund.PlayOrderState, updates["play_order_state"] = &state, state
	if len(order.Raw) > 0 {
		raw := string(order.Raw)
		refund.RawResponse, updates["raw_response"] = &raw, raw
	}
	if ev := order.OrderHistory.RefundEvent; ev != nil {
		if ev.RefundReason != "" {
			reason := ev.RefundReason
			refund.PlayRefundReason, updates["play_refund_reason"] = &reason, reason
		}
		if d := ev.RefundDetails; d != nil {
			if v := d.Total.Amount(); v != nil {
				refund.RefundedTotal, updates["refunded_total"] = v, *v
			}
			if v := d.Tax.Amount(); v != nil {
				refund.RefundedTax, updates["refunded_tax"] = v, *v
			}
			if d.Total != nil && d.Total.CurrencyCode != "" {
				cur := d.Total.CurrencyCode
				refund.Currency, updates["currency"] = &cur, cur
			}
		}
	}
	if err := s.db.Model(refund).Updates(updates).Error; err != nil {
		slog.Error("failed to save refund details", "refund_id", refund.ID, "error", err)
	}
}

// SyncRefundDetails re-reads a refund's details from Google, and finishes a
// refund left REQUESTED (Google accepted it but the local update failed)
// once Google reports the order refunded.
func (s *PaymentService) SyncRefundDetails(ctx context.Context, refundID uuid.UUID) (*models.OrderRefund, error) {
	var refund models.OrderRefund
	if err := s.db.First(&refund, "id = ?", refundID).Error; err != nil {
		return nil, err
	}
	s.fillRefundDetails(ctx, &refund)
	if refund.Status == models.RefundStatusRequested && refund.PlayOrderState != nil &&
		*refund.PlayOrderState == "REFUNDED" {
		order, err := models.FindOrderByID(s.db, refund.OrderID)
		if err != nil {
			return nil, err
		}
		if err := s.completeRefund(order, &refund); err != nil {
			return nil, err
		}
	}
	return &refund, nil
}

// recordGoogleRefund records a refund Google made on its own (voided
// purchase or RTDN revocation) once the order has been moved to REFUNDED.
// Best effort: the order state change is what matters for access.
func (s *PaymentService) recordGoogleRefund(order *models.Order, source string, playOrderID string, voided *VoidedPurchase) {
	now := time.Now()
	refund := models.OrderRefund{
		OrderID:        order.ID,
		Provider:       models.OrderProviderGooglePlay,
		Source:         source,
		RefundType:     models.RefundTypeFull,
		Revoked:        true,
		PlayOrderID:    optionalString(playOrderID),
		PurchaseToken:  order.PurchaseToken,
		OrderAmountIdr: order.AmountIdr,
		Status:         models.RefundStatusSucceeded,
		RequestedAt:    now,
		CompletedAt:    &now,
	}
	if voided != nil {
		refund.VoidedSource, refund.VoidedReason = &voided.VoidedSource, &voided.VoidedReason
	}
	if err := s.db.Create(&refund).Error; err != nil {
		slog.Error("failed to record Google-initiated refund", "order_id", order.ID, "source", source, "error", err)
	}
}

// OrderRefundView is a refund record plus the issuing admin's email.
type OrderRefundView struct {
	models.OrderRefund
	AdminEmail *string `json:"admin_email"`
}

// ListRefunds returns an order's refund records, newest first.
func (s *PaymentService) ListRefunds(orderID uuid.UUID) ([]OrderRefundView, error) {
	var refunds []models.OrderRefund
	if err := s.db.Where("order_id = ?", orderID).Order("requested_at DESC").Find(&refunds).Error; err != nil {
		return nil, err
	}
	views := make([]OrderRefundView, len(refunds))
	for i, r := range refunds {
		views[i] = OrderRefundView{OrderRefund: r}
		if r.AdminID != nil {
			var admin models.AdminUser
			if err := s.db.Select("email").First(&admin, "id = ?", *r.AdminID).Error; err == nil {
				email := admin.Email
				views[i].AdminEmail = &email
			}
		}
	}
	return views, nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
