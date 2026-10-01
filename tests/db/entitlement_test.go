package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// The entitlement layer decides what a paying customer may see. Granting too
// much loses revenue; granting too little breaks someone who paid. Until now
// the models package had no tests at all, and its central guarantee —
// "the same purchase can never grant two entitlements" — was asserted only
// against a mocked driver's expectation list, never against the UNIQUE index
// that actually enforces it.

func TestGrantEntitlement_SameUserAndProductTwice_CreatesExactlyOneRow(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)
	order := fixtures.NewPaidOrder(t, db, user, product)

	require.NoError(t, models.GrantEntitlement(db, user.ID, product.ID, &order.ID))
	// A webhook replay, a retried verification, a double-tap — all land here.
	require.NoError(t, models.GrantEntitlement(db, user.ID, product.ID, &order.ID),
		"re-granting must be a no-op, not an error")

	var count int64
	require.NoError(t, db.Model(&models.UserEntitlement{}).
		Where("user_id = ? AND product_id = ?", user.ID, product.ID).
		Count(&count).Error)
	assert.Equal(t, int64(1), count,
		"idempotency must be enforced by the real UNIQUE (user_id, product_id) index")
}

func TestGrantEntitlement_DifferentProducts_AreIndependent(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	first := fixtures.NewProduct(t, db)
	second := fixtures.NewProduct(t, db)

	require.NoError(t, models.GrantEntitlement(db, user.ID, first.ID, nil))
	require.NoError(t, models.GrantEntitlement(db, user.ID, second.ID, nil))

	for _, product := range []*models.Product{first, second} {
		has, err := models.HasEntitlement(db, user.ID, product.ID)
		require.NoError(t, err)
		assert.True(t, has, "each product must be entitled independently")
	}
}

func TestGrantEntitlement_DifferentUsers_DoNotShareAccess(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	buyer := fixtures.NewUser(t, db)
	bystander := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)

	require.NoError(t, models.GrantEntitlement(db, buyer.ID, product.ID, nil))

	has, err := models.HasEntitlement(db, bystander.ID, product.ID)
	require.NoError(t, err)
	assert.False(t, has, "one user's purchase must never entitle another")
}

func TestGrantEntitlement_UnknownProduct_IsRejectedByForeignKey(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	user := fixtures.NewUser(t, db)

	err := models.GrantEntitlement(db, user.ID, uuid.New(), nil)

	// Only a real database can prove this; a mock would happily accept it.
	require.Error(t, err, "an entitlement to a nonexistent product must be rejected")
	assert.Contains(t, err.Error(), "foreign key")
}

func TestGrantEntitlement_UnknownSourceOrder_IsRejectedByForeignKey(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)
	phantomOrder := uuid.New()

	err := models.GrantEntitlement(db, user.ID, product.ID, &phantomOrder)

	require.Error(t, err, "an entitlement must not cite an order that does not exist")
	assert.Contains(t, err.Error(), "foreign key")
}

func TestHasEntitlement_WithoutGrant_ReportsNoAccess(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)

	has, err := models.HasEntitlement(db, user.ID, product.ID)

	require.NoError(t, err)
	assert.False(t, has)
}

func TestHasEntitlement_ExpiredEntitlement_ReportsNoAccess(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)
	past := time.Now().Add(-time.Hour)

	require.NoError(t, db.Create(&models.UserEntitlement{
		UserID:    user.ID,
		ProductID: product.ID,
		StartsAt:  time.Now().Add(-24 * time.Hour),
		ExpiresAt: &past,
	}).Error)

	has, err := models.HasEntitlement(db, user.ID, product.ID)

	require.NoError(t, err)
	assert.False(t, has, "an expired entitlement must not grant access")
}

func TestHasEntitlement_NullExpiry_IsPermanent(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	user := fixtures.NewUser(t, db)
	product := fixtures.NewProduct(t, db)
	fixtures.NewEntitlement(t, db, user, product)

	has, err := models.HasEntitlement(db, user.ID, product.ID)

	require.NoError(t, err)
	assert.True(t, has, "a one-off purchase has no expiry and never lapses")
}

func TestFindPackageItems_ReturnsEveryProductInTheBundle(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	pkg, products := fixtures.NewPackageWithItems(t, db, 3)
	// A second package must not bleed into the first one's items.
	fixtures.NewPackageWithItems(t, db, 2)

	items, err := models.FindPackageItems(db, uuid.MustParse(pkg.ID))

	require.NoError(t, err)
	require.Len(t, items, 3, "a bundle must resolve to exactly its own products")

	got := make(map[uuid.UUID]bool, len(items))
	for _, item := range items {
		got[item.ProductID] = true
	}
	for _, product := range products {
		assert.True(t, got[product.ID], "product %s missing from bundle", product.ID)
	}
}

func TestFindPackageItems_EmptyPackage_ReturnsNothing(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)
	pkg := fixtures.NewPackage(t, db)

	items, err := models.FindPackageItems(db, uuid.MustParse(pkg.ID))

	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestAddPackageItem_SameProductTwice_IsIdempotent(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	pkg := fixtures.NewPackage(t, db)
	product := fixtures.NewProduct(t, db)
	packageID := uuid.MustParse(pkg.ID)

	require.NoError(t, models.AddPackageItem(db, packageID, product.ID))
	require.NoError(t, models.AddPackageItem(db, packageID, product.ID),
		"re-adding a bundled product must be a no-op")

	items, err := models.FindPackageItems(db, packageID)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}
