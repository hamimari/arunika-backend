package services

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func featureColumns() []string {
	return []string{"id", "created_at", "updated_at", "is_deleted", "code", "name", "description", "is_active"}
}

func productColumns() []string {
	return []string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}
}

// ─── Create ─────────────────────────────────────────────────────────────────

func TestProductService_Create_ArCard_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	featureID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "features" WHERE code = $1 ORDER BY "features"."id" LIMIT $2`)).
		WithArgs("AR_CARD", 1).
		WillReturnRows(sqlmock.NewRows(featureColumns()).
			AddRow(featureID, now, now, false, "AR_CARD", "AR Card", "desc", true))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "products"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "product_ar_cards"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	product, err := svc.Create(CreateProductInput{FeatureCode: FeatureCodeArCard, PriceIdr: 0, ArCardID: "card-1"})

	require.NoError(t, err)
	assert.Equal(t, featureID, product.FeatureID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Create_ArCard_MissingID(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	featureID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "features" WHERE code = $1 ORDER BY "features"."id" LIMIT $2`)).
		WithArgs("AR_CARD", 1).
		WillReturnRows(sqlmock.NewRows(featureColumns()).
			AddRow(featureID, now, now, false, "AR_CARD", "AR Card", "desc", true))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "products"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectRollback()

	product, err := svc.Create(CreateProductInput{FeatureCode: FeatureCodeArCard, PriceIdr: 0})

	assert.Error(t, err)
	assert.Nil(t, product)
}

func TestProductService_Create_UnknownFeature(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "features" WHERE code = $1 ORDER BY "features"."id" LIMIT $2`)).
		WithArgs("NOPE", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	product, err := svc.Create(CreateProductInput{FeatureCode: "NOPE"})

	assert.Error(t, err)
	assert.Nil(t, product)
}

// ─── ResolveByArCardID / ResolveByDongengID ────────────────────────────────────

func TestProductService_ResolveByArCardID_Found(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	productID := uuid.New()
	featureID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_ar_cards" WHERE ar_card_id = $1 ORDER BY "product_ar_cards"."product_id" LIMIT $2`)).
		WithArgs("card-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}).AddRow(productID, "card-1"))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(productID, 1).
		WillReturnRows(sqlmock.NewRows(productColumns()).AddRow(productID, featureID, 0, true, now, now))

	product, err := svc.ResolveByArCardID("card-1")

	require.NoError(t, err)
	require.NotNil(t, product)
	assert.Equal(t, productID, product.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_ResolveByArCardID_FreeContent(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_ar_cards" WHERE ar_card_id = $1 ORDER BY "product_ar_cards"."product_id" LIMIT $2`)).
		WithArgs("free-card", 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}))

	product, err := svc.ResolveByArCardID("free-card")

	require.NoError(t, err)
	assert.Nil(t, product)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_ResolveByDongengID_FreeContent(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	dongengID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_dongengs" WHERE dongeng_id = $1 ORDER BY "product_dongengs"."product_id" LIMIT $2`)).
		WithArgs(dongengID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "dongeng_id"}))

	product, err := svc.ResolveByDongengID(dongengID)

	require.NoError(t, err)
	assert.Nil(t, product)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GetByID ────────────────────────────────────────────────────────────────

func TestProductService_GetByID_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	productID := uuid.New()
	featureID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(productID, 1).
		WillReturnRows(sqlmock.NewRows(productColumns()).AddRow(productID, featureID, 29000, true, now, now))

	product, err := svc.GetByID(productID)

	require.NoError(t, err)
	require.NotNil(t, product)
	assert.Equal(t, productID, product.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_GetByID_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	productID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(productID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	product, err := svc.GetByID(productID)

	assert.Error(t, err)
	assert.Nil(t, product)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── ResolveDisplayName ───────────────────────────────────────────────────────

func TestProductService_ResolveDisplayName_ArCard(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	productID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT a.title FROM product_ar_cards pac JOIN ar_cards a ON a.id = pac.ar_card_id WHERE pac.product_id = $1`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"title"}).AddRow("Singa"))

	name, err := svc.ResolveDisplayName(productID)

	require.NoError(t, err)
	assert.Equal(t, "Singa", name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_ResolveDisplayName_Dongeng(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	productID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT a.title FROM product_ar_cards pac JOIN ar_cards a ON a.id = pac.ar_card_id WHERE pac.product_id = $1`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"title"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT d.title FROM product_dongengs pd JOIN dongengs d ON d.id = pd.dongeng_id WHERE pd.product_id = $1`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"title"}).AddRow("Kancil"))

	name, err := svc.ResolveDisplayName(productID)

	require.NoError(t, err)
	assert.Equal(t, "Kancil", name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_ResolveDisplayName_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	productID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT a.title FROM product_ar_cards pac JOIN ar_cards a ON a.id = pac.ar_card_id WHERE pac.product_id = $1`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"title"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT d.title FROM product_dongengs pd JOIN dongengs d ON d.id = pd.dongeng_id WHERE pd.product_id = $1`)).
		WithArgs(productID).
		WillReturnRows(sqlmock.NewRows([]string{"title"}))

	name, err := svc.ResolveDisplayName(productID)

	assert.Error(t, err)
	assert.Empty(t, name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── List ───────────────────────────────────────────────────────────────────

func TestProductService_List_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	id1 := uuid.New()
	featureID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" ORDER BY created_at desc`)).
		WillReturnRows(sqlmock.NewRows(productColumns()).AddRow(id1, featureID, 29000, true, now, now))

	products, err := svc.List()

	require.NoError(t, err)
	assert.Len(t, products, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── ListEnriched ───────────────────────────────────────────────────────────

func TestProductService_ListEnriched_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	id1 := uuid.New()
	featureID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" ORDER BY created_at desc`)).
		WillReturnRows(sqlmock.NewRows(productColumns()).AddRow(id1, featureID, 29000, true, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "features" WHERE id = $1 ORDER BY "features"."id" LIMIT $2`)).
		WithArgs(featureID, 1).
		WillReturnRows(sqlmock.NewRows(featureColumns()).
			AddRow(featureID, now, now, false, "AR_CARD", "AR Card", "desc", true))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT a.title FROM product_ar_cards pac JOIN ar_cards a ON a.id = pac.ar_card_id WHERE pac.product_id = $1`)).
		WithArgs(id1).
		WillReturnRows(sqlmock.NewRows([]string{"title"}).AddRow("Singa"))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_ar_cards" WHERE product_id = $1 ORDER BY "product_ar_cards"."product_id" LIMIT $2`)).
		WithArgs(id1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "ar_card_id"}).AddRow(id1, "card-1"))

	views, err := svc.ListEnriched()

	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, "AR_CARD", views[0].FeatureCode)
	assert.Equal(t, "card-1", views[0].ContentID)
	assert.Equal(t, "Singa", views[0].DisplayName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── UpdatePrice ────────────────────────────────────────────────────────────

func TestProductService_UpdatePrice_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	id := uuid.New()
	featureID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "products" SET "price_idr"=$1,"updated_at"=$2 WHERE id = $3`)).
		WithArgs(int64(39000), sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows(productColumns()).AddRow(id, featureID, 39000, true, now, now))

	product, err := svc.UpdatePrice(id, 39000)

	require.NoError(t, err)
	assert.Equal(t, int64(39000), product.PriceIdr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── ToggleActive ───────────────────────────────────────────────────────────

func TestProductService_ToggleActive_Deactivate(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "products" SET "is_active"=$1,"updated_at"=$2 WHERE id = $3`)).
		WithArgs(false, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ToggleActive(id, false)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── Delete ─────────────────────────────────────────────────────────────────

func TestProductService_Delete_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "product_ar_cards" WHERE product_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "product_dongengs" WHERE product_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "products" WHERE id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.Delete(id)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestProductService_Delete_StillBundledInPackage(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewProductService(gormDB)

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "product_ar_cards" WHERE product_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "product_dongengs" WHERE product_id = $1`)).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "products" WHERE id = $1`)).
		WithArgs(id).
		WillReturnError(&pgconn.PgError{Code: "23503"})
	mock.ExpectRollback()

	err := svc.Delete(id)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "still bundled")
	assert.NoError(t, mock.ExpectationsWereMet())
}
