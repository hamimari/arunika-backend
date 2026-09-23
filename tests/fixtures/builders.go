package fixtures

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"arunika_backend/models"
)

// Builders construct valid domain objects with sensible defaults and explicit
// overrides, so a test states only what it actually cares about. When a
// migration adds a required column, one builder changes instead of every
// test that happens to touch that table.

// seq makes generated values unique across a package run without the caller
// having to invent names — important because emails and phone numbers carry
// UNIQUE constraints that real PostgreSQL actually enforces.
var seq atomic.Int64

func nextSeq() int64 { return seq.Add(1) }

// ─── Users ────────────────────────────────────────────────────────────────────

type UserOption func(*models.Parent)

func WithEmail(email string) UserOption {
	return func(p *models.Parent) { p.EmailAddress = email }
}

func WithPhone(phone string) UserOption {
	return func(p *models.Parent) { p.PhoneNumber = phone }
}

func WithEmailVerified(verified bool) UserOption {
	return func(p *models.Parent) { p.EmailVerified = verified }
}

func Deleted() UserOption {
	return func(p *models.Parent) { p.IsDeleted = true }
}

// NewUser inserts a parent account and returns it.
func NewUser(t *testing.T, db *gorm.DB, opts ...UserOption) *models.Parent {
	t.Helper()
	n := nextSeq()
	user := &models.Parent{
		Name:          fmt.Sprintf("Test User %d", n),
		PhoneNumber:   fmt.Sprintf("0811%08d", n),
		EmailAddress:  fmt.Sprintf("user%d@example.test", n),
		Password:      "$2a$10$notarealhashbutlongenoughtolooklikeone",
		Address:       "Jl. Test",
		City:          "Jakarta",
		EmailVerified: true,
	}
	for _, opt := range opts {
		opt(user)
	}
	require.NoError(t, db.Create(user).Error, "create user")
	return user
}

// ─── Catalog ──────────────────────────────────────────────────────────────────

// FeatureID returns the id of a seeded feature taxonomy row (AR_CARD or
// DONGENG), inserting it if the migration seed is not present.
func FeatureID(t *testing.T, db *gorm.DB, code string) uuid.UUID {
	t.Helper()
	var feature models.Feature
	err := db.Where("code = ?", code).First(&feature).Error
	if err == nil {
		return feature.ID
	}
	feature = models.Feature{Code: code, Name: code, IsActive: true}
	require.NoError(t, db.Create(&feature).Error, "create feature %s", code)
	return feature.ID
}

type ProductOption func(*models.Product)

func WithPrice(idr int64) ProductOption {
	return func(p *models.Product) { p.PriceIdr = idr }
}

func WithPlayProductID(id string) ProductOption {
	return func(p *models.Product) { p.PlayProductID = &id }
}

func InactiveProduct() ProductOption {
	return func(p *models.Product) { p.IsActive = false }
}

// NewProduct inserts a sellable product under the given feature code
// (defaults to AR_CARD).
func NewProduct(t *testing.T, db *gorm.DB, opts ...ProductOption) *models.Product {
	t.Helper()
	product := &models.Product{
		FeatureID: FeatureID(t, db, "AR_CARD"),
		PriceIdr:  25_000,
		IsActive:  true,
	}
	for _, opt := range opts {
		opt(product)
	}
	require.NoError(t, db.Create(product).Error, "create product")
	return product
}

type PackageOption func(*models.PremiumPackage)

func AsSubscription(durationDays int) PackageOption {
	return func(p *models.PremiumPackage) {
		p.Type = "subscription"
		p.DurationDays = &durationDays
	}
}

func WithPackagePlayProductID(id string) PackageOption {
	return func(p *models.PremiumPackage) { p.PlayProductID = &id }
}

func WithPackagePrice(idr int) PackageOption {
	return func(p *models.PremiumPackage) { p.PriceIdr = idr }
}

// NewPackage inserts a premium package. It defaults to a one-off "content"
// package; pass AsSubscription to make it a recurring plan.
func NewPackage(t *testing.T, db *gorm.DB, opts ...PackageOption) *models.PremiumPackage {
	t.Helper()
	n := nextSeq()
	pkg := &models.PremiumPackage{
		Name:     fmt.Sprintf("Test Package %d", n),
		Subtitle: "For tests",
		PriceIdr: 99_000,
		Type:     "content",
		IsActive: true,
	}
	for _, opt := range opts {
		opt(pkg)
	}
	require.NoError(t, db.Create(pkg).Error, "create package")
	return pkg
}

// NewPackageWithItems inserts a content package containing n freshly created
// products, and returns both. Bundle fan-out is the behaviour most worth
// testing, and hand-wiring package items in every test obscures it.
func NewPackageWithItems(t *testing.T, db *gorm.DB, n int, opts ...PackageOption) (*models.PremiumPackage, []*models.Product) {
	t.Helper()
	pkg := NewPackage(t, db, opts...)
	packageID := uuid.MustParse(pkg.ID)

	products := make([]*models.Product, 0, n)
	for i := 0; i < n; i++ {
		product := NewProduct(t, db)
		require.NoError(t, db.Create(&models.PremiumPackageItem{
			PackageID: packageID,
			ProductID: product.ID,
		}).Error, "link product to package")
		products = append(products, product)
	}
	return pkg, products
}

// ─── Orders ───────────────────────────────────────────────────────────────────

type OrderOption func(*models.Order)

func WithStatus(status string) OrderOption {
	return func(o *models.Order) { o.Status = status }
}

func WithAmount(idr int64) OrderOption {
	return func(o *models.Order) { o.AmountIdr = idr }
}

func WithProvider(provider string) OrderOption {
	return func(o *models.Order) { o.Provider = provider }
}

func WithPurchaseToken(token string) OrderOption {
	return func(o *models.Order) { o.PurchaseToken = &token }
}

// NewOrderForProduct inserts an order referencing a single product. The
// orders table enforces that exactly one of product_id/package_id is set.
func NewOrderForProduct(t *testing.T, db *gorm.DB, user *models.Parent, product *models.Product, opts ...OrderOption) *models.Order {
	t.Helper()
	productID := product.ID
	order := &models.Order{
		UserID:    user.ID,
		ProductID: &productID,
		AmountIdr: product.PriceIdr,
		Status:    "PENDING",
		Provider:  "midtrans",
	}
	for _, opt := range opts {
		opt(order)
	}
	require.NoError(t, db.Create(order).Error, "create order for product")
	return order
}

// NewOrderForPackage inserts an order referencing a package.
func NewOrderForPackage(t *testing.T, db *gorm.DB, user *models.Parent, pkg *models.PremiumPackage, opts ...OrderOption) *models.Order {
	t.Helper()
	packageID := uuid.MustParse(pkg.ID)
	order := &models.Order{
		UserID:    user.ID,
		PackageID: &packageID,
		AmountIdr: int64(pkg.PriceIdr),
		Status:    "PENDING",
		Provider:  "midtrans",
	}
	for _, opt := range opts {
		opt(order)
	}
	require.NoError(t, db.Create(order).Error, "create order for package")
	return order
}

// NewPaidOrder is the common case for entitlement tests: an order that has
// already settled.
func NewPaidOrder(t *testing.T, db *gorm.DB, user *models.Parent, product *models.Product) *models.Order {
	t.Helper()
	return NewOrderForProduct(t, db, user, product, WithStatus("PAID"))
}

// ─── Access ───────────────────────────────────────────────────────────────────

// NewEntitlement grants a user access to a product directly, bypassing the
// purchase flow — for tests about what access *means*, not how it was earned.
func NewEntitlement(t *testing.T, db *gorm.DB, user *models.Parent, product *models.Product) *models.UserEntitlement {
	t.Helper()
	ent := &models.UserEntitlement{
		UserID:    user.ID,
		ProductID: product.ID,
		StartsAt:  time.Now(),
	}
	require.NoError(t, db.Create(ent).Error, "create entitlement")
	return ent
}

type SubscriptionOption func(*models.UserSubscription)

func ExpiringAt(at time.Time) SubscriptionOption {
	return func(s *models.UserSubscription) { s.ExpiresAt = &at }
}

func WithSubscriptionStatus(status string) SubscriptionOption {
	return func(s *models.UserSubscription) { s.Status = status }
}

// NewSubscription gives a user blanket premium access, active for a year by
// default. Pass ExpiringAt with a past time to model a lapsed subscription.
func NewSubscription(t *testing.T, db *gorm.DB, user *models.Parent, opts ...SubscriptionOption) *models.UserSubscription {
	t.Helper()
	expiry := time.Now().Add(365 * 24 * time.Hour)
	now := time.Now()
	sub := &models.UserSubscription{
		UserID:    user.ID,
		Status:    "premium",
		ExpiresAt: &expiry,
		StartDate: &now,
	}
	for _, opt := range opts {
		opt(sub)
	}
	require.NoError(t, db.Create(sub).Error, "create subscription")
	return sub
}

// ─── Content ──────────────────────────────────────────────────────────────────

type DongengOption func(*models.Dongeng)

func WithTitle(title string) DongengOption {
	return func(d *models.Dongeng) { d.Title = title }
}

func Hidden() DongengOption {
	return func(d *models.Dongeng) { d.Hidden = true }
}

func SoftDeleted() DongengOption {
	return func(d *models.Dongeng) { d.IsDeleted = true }
}

func Paid() DongengOption {
	return func(d *models.Dongeng) { d.IsFree = false }
}

func InCategory(categoryID uuid.UUID) DongengOption {
	return func(d *models.Dongeng) { d.DongengCategoryID = &categoryID }
}

// NewDongeng inserts a visible, free fairy tale.
func NewDongeng(t *testing.T, db *gorm.DB, opts ...DongengOption) *models.Dongeng {
	t.Helper()
	n := nextSeq()
	dongeng := &models.Dongeng{
		Title:    fmt.Sprintf("Dongeng %d", n),
		AgeStart: 3,
		AgeEnd:   6,
		ImageUrl: "https://example.test/image.png",
		AudioUrl: "https://example.test/audio.mp3",
		IsFree:   true,
		Duration: 300,
	}
	for _, opt := range opts {
		opt(dongeng)
	}
	require.NoError(t, db.Create(dongeng).Error, "create dongeng")
	return dongeng
}

// NewDongengCategory inserts a top-level dongeng category.
func NewDongengCategory(t *testing.T, db *gorm.DB) *models.DongengCategory {
	t.Helper()
	n := nextSeq()
	category := &models.DongengCategory{Name: fmt.Sprintf("Category %d", n)}
	require.NoError(t, db.Create(category).Error, "create dongeng category")
	return category
}

// NextSeq exposes the shared counter so helpers outside this package can
// generate values that satisfy UNIQUE constraints.
func NextSeq() int64 { return nextSeq() }
