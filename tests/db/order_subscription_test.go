package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/services"
	"arunika_backend/tests/fixtures"
)

// ─── Orders ───────────────────────────────────────────────────────────────────

func TestOrder_MustReferenceExactlyOneOfProductOrPackage(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)
	pkg := fixtures.NewPackage(t, db)
	packageID := uuid.MustParse(pkg.ID)
	productID := product.ID

	t.Run("neither is rejected", func(t *testing.T) {
		err := db.Create(&models.Order{
			UserID: user.ID, AmountIdr: 1000, Status: "PENDING", Provider: "midtrans",
		}).Error
		require.Error(t, err, "an order must buy something")
	})

	t.Run("both is rejected", func(t *testing.T) {
		err := db.Create(&models.Order{
			UserID: user.ID, ProductID: &productID, PackageID: &packageID,
			AmountIdr: 1000, Status: "PENDING", Provider: "midtrans",
		}).Error
		require.Error(t, err, "an order must not buy a product and a package at once")
	})
}

func TestOrder_StatusIsConstrainedToKnownValues(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)

	for _, status := range []string{"PENDING", "PAID", "FAILED", "EXPIRED", "REFUNDED"} {
		t.Run("accepts "+status, func(t *testing.T) {
			order := fixtures.NewOrderForProduct(t, db, user, product, fixtures.WithStatus(status))
			assert.Equal(t, status, order.Status)
		})
	}

	t.Run("rejects an unknown status", func(t *testing.T) {
		productID := product.ID
		err := db.Create(&models.Order{
			UserID: user.ID, ProductID: &productID, AmountIdr: 1000,
			Status: "SETTLED", Provider: "midtrans",
		}).Error
		require.Error(t, err, "a typo'd status must not reach the table")
	})
}

func TestOrder_ProviderIsConstrainedToKnownRails(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)
	productID := product.ID

	err := db.Create(&models.Order{
		UserID: user.ID, ProductID: &productID, AmountIdr: 1000,
		Status: "PENDING", Provider: "stripe",
	}).Error

	require.Error(t, err, "only midtrans and google_play are valid payment rails")
}

func TestOrder_UnknownUser_IsRejectedByForeignKey(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	product := fixtures.NewProduct(t, db)
	productID := product.ID

	err := db.Create(&models.Order{
		UserID: uuid.New(), ProductID: &productID, AmountIdr: 1000,
		Status: "PENDING", Provider: "midtrans",
	}).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "foreign key")
}

// Purchase-token reuse is what stops one Play purchase unlocking content for
// several accounts. This documents where that guarantee actually lives: in
// application code (PaymentService.VerifyPlayPurchase), NOT in the schema.
// There is no unique index on orders.purchase_token.
func TestOrder_PurchaseTokenReuse_IsNotPreventedByTheSchema(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	buyer := fixtures.NewUser(t, db)
	other := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)

	fixtures.NewOrderForProduct(t, db, buyer, product,
		fixtures.WithProvider("google_play"), fixtures.WithPurchaseToken("token-shared"))

	err := db.Create(&models.Order{
		UserID: other.ID, ProductID: &product.ID, AmountIdr: product.PriceIdr,
		Status: "PENDING", Provider: "google_play",
		PurchaseToken: func() *string { s := "token-shared"; return &s }(),
	}).Error

	assert.NoError(t, err,
		"the database accepts a duplicate purchase token — the defence is in "+
			"PaymentService.VerifyPlayPurchase, so it must never be removed from there")
}

// ─── Subscriptions ────────────────────────────────────────────────────────────

// user_subscriptions.user_id is UNIQUE. This is the constraint behind the
// deadlock warning on EntitlementService.SyncSubscriptionExpiry: a second
// connection inserting for the same user blocks on this index while the
// transaction holding it waits on the second connection.
func TestSubscription_OneRowPerUser(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, user)

	expiry := time.Now().Add(24 * time.Hour)
	err := db.Create(&models.UserSubscription{
		UserID: user.ID, Status: "premium", ExpiresAt: &expiry,
	}).Error

	require.Error(t, err, "a user must never have two subscription rows")
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestSubscription_StatusIsConstrainedToKnownValues(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	for _, status := range []string{"free", "premium"} {
		t.Run("accepts "+status, func(t *testing.T) {
			user := fixtures.NewUser(t, db)
			sub := fixtures.NewSubscription(t, db, user, fixtures.WithSubscriptionStatus(status))
			assert.Equal(t, status, sub.Status)
		})
	}
}

// Regression: RevokeSubscription used to write status='revoked', which the
// schema's CHECK (status IN ('free','premium')) rejected with SQLSTATE 23514
// — so every refund and Play revocation silently failed and the user kept
// premium access. The sqlmock test covering it passed, because a mocked
// driver does not enforce CHECK constraints. This is the test that caught it.
func TestSubscription_RevokeSubscription_SucceedsAgainstRealSchema(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, user)

	err := services.NewEntitlementService(db).RevokeSubscription(user.ID)

	require.NoError(t, err,
		"revoking a refunded subscription must actually work against the real schema")

	var sub models.UserSubscription
	require.NoError(t, db.Where("user_id = ?", user.ID).First(&sub).Error)
	assert.Equal(t, "free", sub.Status, "a revoked subscription must not stay premium")
	require.NotNil(t, sub.ExpiresAt)
	assert.False(t, sub.ExpiresAt.After(time.Now()),
		"revocation ends access immediately, unlike a cancellation")
}
