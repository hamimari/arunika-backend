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
		return s.upsertSubscription(tx, order.UserID, *order.PackageID, *pkg.DurationDays)
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
func (s *EntitlementService) upsertSubscription(tx *gorm.DB, userID, packageID uuid.UUID, durationDays int) error {
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
	}).Error
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
