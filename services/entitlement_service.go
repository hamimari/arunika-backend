package services

import (
	"arunika_backend/models"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type EntitlementService struct {
	db *gorm.DB
}

// RenewalWindowDays is how long before expiry a subscription that won't
// renew by itself may be renewed in-app. The new period is added on top of
// the current expiry (see computeSubscriptionExpiry), so renewing early
// never costs the user paid days.
const RenewalWindowDays = 7

// ErrSubscriptionActive is returned when a user with an active subscription
// tries to start a purchase they don't need — they already have access to
// all paid content. Handlers answer it with 409 SUBSCRIPTION_ACTIVE.
var ErrSubscriptionActive = errors.New("subscription is already active")

// SubscriptionRenewal describes whether sub is active and whether it may be
// renewed in-app right now: only when it is active, won't auto-renew, and
// is within RenewalWindowDays of expiry. A subscription with no expiry
// (admin manual grant) is never renewable.
type SubscriptionRenewal struct {
	Active        bool
	CanRenew      bool
	RenewableFrom *time.Time
}

func RenewalFor(sub *models.UserSubscription, now time.Time) SubscriptionRenewal {
	if sub == nil || sub.Status != "premium" {
		return SubscriptionRenewal{}
	}
	if sub.ExpiresAt == nil {
		return SubscriptionRenewal{Active: true}
	}
	if !sub.ExpiresAt.After(now) {
		return SubscriptionRenewal{}
	}
	from := sub.ExpiresAt.AddDate(0, 0, -RenewalWindowDays)
	return SubscriptionRenewal{
		Active:        true,
		CanRenew:      !sub.AutoRenew && !now.Before(from),
		RenewableFrom: &from,
	}
}

// CheckPurchaseAllowed returns ErrSubscriptionActive when userID has an
// active subscription, unless this is an in-app renewal: a subscription
// package (pkg non-nil, type subscription), paid outside Google Play
// (viaPlay false — Play subscriptions renew through Play itself), for a
// non-Play subscription inside its renewal window. pkg is nil for a
// single-product purchase.
func (s *EntitlementService) CheckPurchaseAllowed(userID uuid.UUID, pkg *models.PremiumPackage, viaPlay bool) error {
	var sub models.UserSubscription
	err := s.db.Where("user_id = ?", userID).First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	renewal := RenewalFor(&sub, time.Now())
	if !renewal.Active {
		return nil
	}
	isRenewal := pkg != nil && pkg.Type == "subscription" && !viaPlay &&
		renewal.CanRenew && sub.Provider != models.OrderProviderGooglePlay
	if isRenewal {
		return nil
	}
	return ErrSubscriptionActive
}

func NewEntitlementService(db *gorm.DB) *EntitlementService {
	return &EntitlementService{db: db}
}

// GrantForPaidOrder grants access for a newly-PAID order. Callers running
// this as part of the payment webhook MUST pass the same transaction that
// flips the order to PAID, and must only call this once per order (guarded
// by the caller checking the order was still PENDING) — grants themselves
// are idempotent (safe to re-run), but re-running always re-derives the
// same, already-correct state.
func (s *EntitlementService) GrantForPaidOrder(tx *gorm.DB, order *models.Order) error {
	switch {
	case order.PackageID != nil:
		return s.grantForPackage(tx, order)
	case order.ProductID != nil:
		return models.GrantEntitlement(tx, order.UserID, *order.ProductID, &order.ID)
	default:
		return errors.New("order has neither product_id nor package_id")
	}
}

func (s *EntitlementService) grantForPackage(tx *gorm.DB, order *models.Order) error {
	pkg, err := models.FindPremiumPackageByID(tx, order.PackageID.String())
	if err != nil {
		return err
	}

	switch pkg.Type {
	case "content":
		items, err := models.FindPackageItems(tx, *order.PackageID)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := models.GrantEntitlement(tx, order.UserID, item.ProductID, &order.ID); err != nil {
				return err
			}
		}
		return nil
	case "subscription":
		if pkg.DurationDays == nil {
			return errors.New("subscription package missing duration_days")
		}
		return s.upsertSubscription(tx, order.UserID, *order.PackageID, *pkg.DurationDays, orderProvider(order))
	default:
		return errors.New("unknown package type: " + pkg.Type)
	}
}

// upsertSubscription grants blanket subscription access, using the
// package's own duration_days (30 for monthly, 365 for yearly, etc.).
// Renewing/extending an already-active subscription adds the new duration
// on top of its current expires_at, rather than resetting from now — buying
// early never costs the user days already paid for. A lapsed subscription
// (or a brand-new one) extends from now instead, since there is no unused
// remainder to preserve.
func (s *EntitlementService) upsertSubscription(tx *gorm.DB, userID, packageID uuid.UUID, durationDays int, provider string) error {
	now := time.Now()

	var sub models.UserSubscription
	err := tx.Where("user_id = ?", userID).First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			expiry := now.AddDate(0, 0, durationDays)
			sub = models.UserSubscription{
				UserID:    userID,
				Status:    "premium",
				ExpiresAt: &expiry,
				PackageID: &packageID,
				StartDate: &now,
				Provider:  provider,
			}
			return tx.Create(&sub).Error
		}
		return err
	}

	expiry := computeSubscriptionExpiry(now, sub.ExpiresAt, durationDays)

	return tx.Model(&sub).Updates(map[string]interface{}{
		"status":     "premium",
		"expires_at": expiry,
		"package_id": packageID,
		"start_date": now,
		"provider":   provider,
	}).Error
}

// orderProvider is the payment rail an order was placed on, defaulting to
// Midtrans for orders predating the provider column.
func orderProvider(order *models.Order) string {
	if order.Provider == "" {
		return models.OrderProviderMidtrans
	}
	return order.Provider
}

// computeSubscriptionExpiry returns the new expires_at for a subscription
// purchase: extends from whichever is later, now or the current expiry (if
// the subscription is still active), by durationDays. A lapsed or absent
// subscription has nothing to preserve, so it simply extends from now.
func computeSubscriptionExpiry(now time.Time, currentExpiry *time.Time, durationDays int) time.Time {
	base := now
	if currentExpiry != nil && currentExpiry.After(now) {
		base = *currentExpiry
	}
	return base.AddDate(0, 0, durationDays)
}

// HasAccess reports whether userID can access productID, via an active
// subscription (blanket access) or a specific entitlement. Callers must
// first confirm the content item actually has a linked product — content
// with no linked product is free and should never reach this check.
func (s *EntitlementService) HasAccess(userID uuid.UUID, productID uuid.UUID) (bool, error) {
	hasSub, err := s.hasActiveSubscription(userID)
	if err != nil {
		return false, err
	}
	if hasSub {
		return true, nil
	}
	return models.HasEntitlement(s.db, userID, productID)
}

func (s *EntitlementService) hasActiveSubscription(userID uuid.UUID) (bool, error) {
	var sub models.UserSubscription
	err := s.db.Where("user_id = ?", userID).First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return sub.Status == "premium" && (sub.ExpiresAt == nil || sub.ExpiresAt.After(time.Now())), nil
}

// SyncSubscriptionExpiry sets a user's subscription expiry to exactly what
// Google Play reports (rather than extending by a duration, as a fresh
// purchase does), since Play is the source of truth once a subscription is
// under Play Billing management — used to reconcile Real-time Developer
// Notifications (renewal, recovery, restart) after the initial purchase.
//
// tx must be the caller's transaction whenever one is open: user_id is
// UNIQUE, so running this on a separate connection while an uncommitted
// upsertSubscription insert holds that index deadlocks — this blocks on the
// index, and the transaction holding it blocks on this returning.
func (s *EntitlementService) SyncSubscriptionExpiry(tx *gorm.DB, userID, packageID uuid.UUID, expiresAt time.Time, autoRenew bool) error {
	var sub models.UserSubscription
	err := tx.Where("user_id = ?", userID).First(&sub).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sub = models.UserSubscription{
				UserID:    userID,
				PackageID: &packageID,
			}
			sub.Status = "premium"
			sub.ExpiresAt = &expiresAt
			now := time.Now()
			sub.StartDate = &now
			sub.Provider = models.OrderProviderGooglePlay
			sub.AutoRenew = autoRenew
			return tx.Create(&sub).Error
		}
		return err
	}
	return tx.Model(&sub).Updates(map[string]interface{}{
		"status":     "premium",
		"expires_at": expiresAt,
		"package_id": packageID,
		"provider":   models.OrderProviderGooglePlay,
		"auto_renew": autoRenew,
	}).Error
}

// SetAutoRenew records whether a Google Play subscription will renew by
// itself — false after the user cancels in Play (access continues until
// expires_at), true again once they resubscribe.
func (s *EntitlementService) SetAutoRenew(userID uuid.UUID, autoRenew bool) error {
	return s.db.Model(&models.UserSubscription{}).
		Where("user_id = ?", userID).
		Update("auto_renew", autoRenew).Error
}

// RevokeSubscription immediately ends a user's subscription access — used
// when Google Play reports a revocation or refund (as opposed to a
// cancellation, which just turns off auto-renew and lets the current period
// run out naturally).
func (s *EntitlementService) RevokeSubscription(userID uuid.UUID) error {
	// Writes 'free', not 'revoked'. user_subscriptions carries
	// CHECK (status IN ('free','premium')), so the previous 'revoked' value
	// failed with SQLSTATE 23514 every time — meaning refunds and Play
	// revocations silently left the user premium. The sqlmock test covering
	// this passed because a mocked driver does not enforce CHECK constraints.
	//
	// Nothing ever read status='revoked' (it was write-only), and the refund
	// audit trail lives on orders.status='REFUNDED' and the payments table,
	// which is where it belongs. Setting expires_at to now preserves the
	// documented behaviour that revocation ends access immediately, rather
	// than letting the paid period run out as a cancellation would.
	return s.db.Model(&models.UserSubscription{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"status":     "free",
			"expires_at": time.Now(),
		}).Error
}

// RevokeEntitlementForOrder immediately ends any content-pack/product
// entitlements granted by orderID — used when Google Play reports the
// purchase behind that order as voided (refund, chargeback, or Google's own
// automatic refund of a purchase left unacknowledged for 3 days).
func (s *EntitlementService) RevokeEntitlementForOrder(orderID uuid.UUID) error {
	return s.db.Model(&models.UserEntitlement{}).
		Where("source_order_id = ?", orderID).
		Update("expires_at", time.Now()).Error
}
