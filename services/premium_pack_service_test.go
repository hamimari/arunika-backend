package services

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func premiumPackColumns() []string {
	return []string{
		"id", "name", "subtitle", "price_idr", "type",
		"badge_label", "is_best_value", "is_active", "sort_order",
		"created_at", "updated_at",
	}
}

// ─── GetAllPacks ───────────────────────────────────────────────────────────────

func TestPremiumPackService_GetAllPacks_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	id1 := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow(id1, "Basic", "Akses konten dasar", 49000, "content",
			nil, false, true, 1, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" ORDER BY sort_order asc`,
	)).WillReturnRows(rows)

	packs, err := svc.GetAllPacks()

	require.NoError(t, err)
	assert.Len(t, packs, 1)
	assert.Equal(t, "Basic", packs[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPremiumPackService_GetAllPacks_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" ORDER BY sort_order asc`,
	)).WillReturnError(gorm.ErrInvalidDB)

	packs, err := svc.GetAllPacks()

	assert.Error(t, err)
	assert.Nil(t, packs)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GetActivePacks ────────────────────────────────────────────────────────────

func TestPremiumPackService_GetActivePacks_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	id1 := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow(id1, "Premium", "Akses penuh", 99000, "subscription",
			nil, true, true, 0, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" WHERE is_active = true ORDER BY sort_order asc`,
	)).WillReturnRows(rows)

	packs, err := svc.GetActivePacks("", nil)

	require.NoError(t, err)
	assert.Len(t, packs, 1)
	assert.Equal(t, "Premium", packs[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPremiumPackService_GetActivePacks_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" WHERE is_active = true ORDER BY sort_order asc`,
	)).WillReturnError(gorm.ErrInvalidDB)

	packs, err := svc.GetActivePacks("", nil)

	assert.Error(t, err)
	assert.Nil(t, packs)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPremiumPackService_GetActivePacks_ExcludesPurchasedContentPacks(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	userID := uuid.New()
	boughtID := uuid.New().String()
	unboughtID := uuid.New().String()
	subID := uuid.New().String()
	now := time.Now()

	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow(boughtID, "Paket Hutan", "sudah dibeli", 29000, "content", nil, false, true, 1, now, now).
		AddRow(unboughtID, "Paket Laut", "belum dibeli", 29000, "content", nil, false, true, 2, now, now).
		AddRow(subID, "Bulanan", "langganan", 39000, "subscription", nil, false, true, 3, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" WHERE is_active = true ORDER BY sort_order asc`,
	)).WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "package_id" FROM "orders" WHERE user_id = $1 AND status = $2 AND package_id IS NOT NULL`)).
		WithArgs(userID, "PAID").
		WillReturnRows(sqlmock.NewRows([]string{"package_id"}).AddRow(boughtID))

	packs, err := svc.GetActivePacks("", &userID)

	require.NoError(t, err)
	require.Len(t, packs, 2)
	names := []string{packs[0].Name, packs[1].Name}
	assert.ElementsMatch(t, []string{"Paket Laut", "Bulanan"}, names)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPremiumPackService_GetActivePacks_PurchasedLookupError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	userID := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows(premiumPackColumns()).
		AddRow(uuid.New().String(), "Paket Hutan", "x", 29000, "content", nil, false, true, 1, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "premium_packages" WHERE is_active = true ORDER BY sort_order asc`,
	)).WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "package_id" FROM "orders" WHERE user_id = $1 AND status = $2 AND package_id IS NOT NULL`)).
		WithArgs(userID, "PAID").
		WillReturnError(gorm.ErrInvalidDB)

	packs, err := svc.GetActivePacks("", &userID)

	assert.Error(t, err)
	assert.Nil(t, packs)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── duration_days validation ──────────────────────────────────────────────────

func TestValidateDurationDays_SubscriptionRequiresDuration(t *testing.T) {
	assert.Error(t, validateDurationDays("subscription", nil))
	zero := 0
	assert.Error(t, validateDurationDays("subscription", &zero))
}

func TestValidateDurationDays_SubscriptionWithDuration_OK(t *testing.T) {
	days := 30
	assert.NoError(t, validateDurationDays("subscription", &days))
}

func TestValidateDurationDays_ContentIgnoresDuration(t *testing.T) {
	assert.NoError(t, validateDurationDays("content", nil))
	days := 30
	assert.NoError(t, validateDurationDays("content", &days))
}

func TestPremiumPackService_CreatePack_SubscriptionMissingDuration_Error(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	pack, err := svc.CreatePack(CreatePremiumPackInput{
		Name: "Bulanan", Subtitle: "Akses 1 bulan", PriceIdr: 39000, Type: "subscription",
	})

	assert.Error(t, err)
	assert.Nil(t, pack)
}

func TestPremiumPackService_UpdatePack_SubscriptionMissingDuration_Error(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	pack, err := svc.UpdatePack("id-5", UpdatePremiumPackInput{
		Name: "Bulanan", Subtitle: "Akses 1 bulan", PriceIDR: 39000, Type: "subscription",
	})

	assert.Error(t, err)
	assert.Nil(t, pack)
}

// ─── Package items ──────────────────────────────────────────────────────────

func TestPremiumPackService_ListItems_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	packageID := uuid.New()
	productID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "premium_package_items" WHERE package_id = $1`)).
		WithArgs(packageID).
		WillReturnRows(sqlmock.NewRows([]string{"package_id", "product_id", "created_at"}).
			AddRow(packageID, productID, now))

	items, err := svc.ListItems(packageID.String())

	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPremiumPackService_ListItems_InvalidID(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	items, err := svc.ListItems("not-a-uuid")

	assert.Error(t, err)
	assert.Nil(t, items)
}

func TestPremiumPackService_AddItem_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	packageID := uuid.New()
	productID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "premium_package_items"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.AddItem(packageID.String(), productID.String())

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPremiumPackService_AddItem_InvalidProductID(t *testing.T) {
	gormDB, _ := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	err := svc.AddItem(uuid.New().String(), "not-a-uuid")

	assert.Error(t, err)
}

func TestPremiumPackService_RemoveItem_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewPremiumPackService(gormDB, NewOrderService(gormDB, NewProductService(gormDB)))

	packageID := uuid.New()
	productID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "premium_package_items" WHERE package_id = $1 AND product_id = $2`)).
		WithArgs(packageID, productID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.RemoveItem(packageID.String(), productID.String())

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
