package db_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/services"
	"arunika_backend/tests/fixtures"
)

// ─── Strike prices (V56) ──────────────────────────────────────────────────────

func TestStrikePriceRules_SeededOff(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	rules, err := models.FindAllStrikePriceRules(db)
	require.NoError(t, err)
	require.Len(t, rules, 3)
	for _, r := range rules {
		assert.Equal(t, models.StrikeModeNone, r.Mode, "every scope starts with no promo: %s", r.Scope)
	}
}

func TestStrikePrice_ActivePromoRequiresEndDate(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	err := db.Exec(`UPDATE strike_price_rules SET mode = 'PERCENT', value = 20 WHERE scope = 'PACKAGE'`).Error
	require.Error(t, err, "a global promo without ends_at is rejected")

	product := fixtures.NewProduct(t, db)
	err = db.Exec(`UPDATE products SET strike_mode = 'FIXED', strike_value = 5000 WHERE id = ?`, product.ID).Error
	require.Error(t, err, "an item promo without strike_ends_at is rejected")

	err = db.Exec(`UPDATE products SET strike_mode = 'NONE' WHERE id = ?`, product.ID).Error
	require.NoError(t, err, "opting out needs no end date")
}

func TestStrikePrice_PackagesShowPromoButOrdersChargeRealPrice(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	strike := services.NewStrikePriceService(db)
	products := services.NewProductService(db)
	packs := services.NewPremiumPackService(db, services.NewOrderService(db, products)).WithStrikePricing(strike)

	pkg := fixtures.NewPackage(t, db, fixtures.WithPackagePrice(79000), fixtures.WithPackagePlayProductID("pkg_hutan"))
	end := time.Now().AddDate(0, 0, 10)
	_, err := strike.UpdateRule(models.StrikeScopePackage, services.UpdateRuleInput{Mode: models.StrikeModePercent, Value: 20, EndsAt: &end})
	require.NoError(t, err)

	active, err := packs.GetActivePacks("", nil)
	require.NoError(t, err)
	var got *models.PremiumPackage
	for i := range active {
		if active[i].ID == pkg.ID {
			got = &active[i]
		}
	}
	require.NotNil(t, got)
	require.NotNil(t, got.StrikePriceIdr)
	assert.Equal(t, int64(99000), *got.StrikePriceIdr)
	assert.Equal(t, 20, *got.DiscountPercent)

	user := fixtures.NewUser(t, db)
	order, err := services.NewPaymentService(db, services.NewEntitlementService(db)).CreatePlayOrder(user, got)
	require.NoError(t, err)
	assert.Equal(t, int64(79000), order.AmountIdr, "the order charges price_idr, never the strike price")
}

// ─── Subscription provider & renewal (V57) ────────────────────────────────────

func TestSubscriptionRenewal_OnlyInsideWindow(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	ent := services.NewEntitlementService(db)
	monthly := fixtures.NewPackage(t, db, fixtures.AsSubscription(30))

	early := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, early, fixtures.ExpiringAt(time.Now().AddDate(0, 0, 20)))
	assert.ErrorIs(t, ent.CheckPurchaseAllowed(early.ID, monthly, false), services.ErrSubscriptionActive)

	due := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, due, fixtures.ExpiringAt(time.Now().AddDate(0, 0, 5)))
	assert.NoError(t, ent.CheckPurchaseAllowed(due.ID, monthly, false))
	assert.ErrorIs(t, ent.CheckPurchaseAllowed(due.ID, nil, false), services.ErrSubscriptionActive,
		"single products stay blocked inside the window")
}

func TestSubscriptionRenewal_StacksAndRecordsProvider(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	ent := services.NewEntitlementService(db)
	yearly := fixtures.NewPackage(t, db, fixtures.AsSubscription(365))
	user := fixtures.NewUser(t, db)
	previous := time.Now().AddDate(0, 0, 5).Truncate(time.Second)
	fixtures.NewSubscription(t, db, user, fixtures.ExpiringAt(previous))

	order := fixtures.NewOrderForPackage(t, db, user, yearly, fixtures.WithStatus(models.OrderStatusPaid))
	require.NoError(t, ent.GrantForPaidOrder(db, order))

	var sub models.UserSubscription
	require.NoError(t, db.Where("user_id = ?", user.ID).First(&sub).Error)
	assert.WithinDuration(t, previous.AddDate(0, 0, 365), *sub.ExpiresAt, time.Second,
		"the new year is added on top of the previous expiry")
	assert.Equal(t, uuid.MustParse(yearly.ID), *sub.PackageID, "plan switches to the renewed package")
	assert.Equal(t, models.OrderProviderMidtrans, sub.Provider)
}

// V57's backfill marks subscriptions whose latest PAID subscription order
// came through Google Play as Play-managed and auto-renewing.
func TestMigrations_V57_BackfillsPlaySubscriptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := fixtures.FreshDB(t)

	playPkg := fixtures.NewPackage(t, db, fixtures.AsSubscription(30))
	playUser := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, playUser)
	fixtures.NewOrderForPackage(t, db, playUser, playPkg,
		fixtures.WithStatus(models.OrderStatusPaid), fixtures.WithProvider(models.OrderProviderGooglePlay))

	midtransUser := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, midtransUser)
	fixtures.NewOrderForPackage(t, db, midtransUser, playPkg, fixtures.WithStatus(models.OrderStatusPaid))

	paths, err := fixtures.VersionedMigrationFiles()
	require.NoError(t, err)
	var v57 string
	for _, p := range paths {
		if strings.HasPrefix(filepath.Base(p), "V57__") {
			v57 = p
		}
	}
	require.NotEmpty(t, v57)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, fixtures.ApplyFile(ctx, sqlDB, v57), "V57 must be safely re-runnable")

	var play, midtrans models.UserSubscription
	require.NoError(t, db.Where("user_id = ?", playUser.ID).First(&play).Error)
	require.NoError(t, db.Where("user_id = ?", midtransUser.ID).First(&midtrans).Error)
	assert.Equal(t, models.OrderProviderGooglePlay, play.Provider)
	assert.True(t, play.AutoRenew)
	assert.Equal(t, models.OrderProviderMidtrans, midtrans.Provider)
	assert.False(t, midtrans.AutoRenew)
}
