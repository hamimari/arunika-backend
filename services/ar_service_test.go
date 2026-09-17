package services

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newArService(db *gorm.DB) *ArService {
	return NewArService(db, NewProductService(db), NewEntitlementService(db))
}

func TestArService_GetByID_Success_FreeContent(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newArService(gormDB)

	id := "ar-card-1"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code", "created_at", "expires_at",
	}).AddRow(id, "model", "Dragon", "https://cdn/dragon.glb", "", "DRG", now, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 AND hidden = $2 ORDER BY "ar_cards"."id" LIMIT $3`)).
		WithArgs(id, false, 1).
		WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_ar_cards" WHERE ar_card_id = $1 ORDER BY "product_ar_cards"."product_id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}))

	result, err := svc.GetByID(id, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, id, result.ID)
	assert.Equal(t, "Dragon", result.Title)
	assert.True(t, result.IsUnlocked)
	assert.Nil(t, result.ProductID)
	assert.Nil(t, result.PriceIdr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestArService_GetByID_PaidContent_Unauthenticated_Locked(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newArService(gormDB)

	id := "ar-card-2"
	productID := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code", "created_at", "expires_at",
	}).AddRow(id, "model", "Dragon", "https://cdn/dragon.glb", "", "DRG", now, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 AND hidden = $2 ORDER BY "ar_cards"."id" LIMIT $3`)).
		WithArgs(id, false, 1).
		WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_ar_cards" WHERE ar_card_id = $1 ORDER BY "product_ar_cards"."product_id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}).AddRow(productID, id))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(productID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}).
			AddRow(productID, uuid.New(), 12000, true, now, now))

	result, err := svc.GetByID(id, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsUnlocked)
	require.NotNil(t, result.ProductID)
	assert.Equal(t, productID, *result.ProductID)
	require.NotNil(t, result.PriceIdr)
	assert.Equal(t, int64(12000), *result.PriceIdr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestArService_GetByID_InactiveProduct_NoEntitlement_Hidden(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newArService(gormDB)

	id := "ar-card-3"
	productID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code", "created_at", "expires_at",
	}).AddRow(id, "model", "Dragon", "https://cdn/dragon.glb", "", "DRG", now, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 AND hidden = $2 ORDER BY "ar_cards"."id" LIMIT $3`)).
		WithArgs(id, false, 1).
		WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_ar_cards" WHERE ar_card_id = $1 ORDER BY "product_ar_cards"."product_id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}).AddRow(productID, id))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(productID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}).
			AddRow(productID, uuid.New(), 12000, false, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "user_entitlements" WHERE user_id = $1 AND product_id = $2 AND (expires_at IS NULL OR expires_at > NOW())`)).
		WithArgs(userID, productID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	result, err := svc.GetByID(id, &userID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsUnlocked)
	assert.Nil(t, result.ProductID)
	assert.Nil(t, result.PriceIdr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestArService_GetByID_InactiveProduct_ExistingOwner_StillUnlocked(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newArService(gormDB)

	id := "ar-card-4"
	productID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "type", "title", "file_url", "sound_url", "short_code", "created_at", "expires_at",
	}).AddRow(id, "model", "Dragon", "https://cdn/dragon.glb", "", "DRG", now, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 AND hidden = $2 ORDER BY "ar_cards"."id" LIMIT $3`)).
		WithArgs(id, false, 1).
		WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_ar_cards" WHERE ar_card_id = $1 ORDER BY "product_ar_cards"."product_id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}).AddRow(productID, id))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(productID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}).
			AddRow(productID, uuid.New(), 12000, false, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "user_entitlements" WHERE user_id = $1 AND product_id = $2 AND (expires_at IS NULL OR expires_at > NOW())`)).
		WithArgs(userID, productID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	result, err := svc.GetByID(id, &userID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsUnlocked)
	require.NotNil(t, result.ProductID)
	assert.Equal(t, productID, *result.ProductID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestArService_GetByID_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newArService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 AND hidden = $2 ORDER BY "ar_cards"."id" LIMIT $3`)).
		WithArgs("missing-id", false, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	result, err := svc.GetByID("missing-id", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestArService_GetByID_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newArService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1 AND hidden = $2 ORDER BY "ar_cards"."id" LIMIT $3`)).
		WithArgs("any-id", false, 1).
		WillReturnError(sql.ErrConnDone)

	result, err := svc.GetByID("any-id", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}
