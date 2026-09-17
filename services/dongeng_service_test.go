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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupMockDB creates an in-memory mock SQL database wrapped by GORM.
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	t.Cleanup(func() { db.Close() })

	dialector := postgres.New(postgres.Config{
		Conn:       db,
		DriverName: "postgres",
	})
	gormDB, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	return gormDB, mock
}

func newDongengService(db *gorm.DB) *DongengService {
	return NewDongengService(db, NewProductService(db), NewEntitlementService(db))
}

// ─── GetFairyTales tests ──────────────────────────────────────────────────────

func dongengColumns() []string {
	return []string{
		"id", "title", "age_start", "age_end", "image_url", "audio_url",
		"is_free", "category_id", "duration", "created_at", "updated_at", "is_deleted",
	}
}

func TestGetFairyTales_ReturnsAllActive(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	id1 := uuid.New()
	id2 := uuid.New()
	now := time.Now()

	// COUNT query
	countRows := sqlmock.NewRows([]string{"count"}).AddRow(2)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2`)).
		WithArgs(false, false).
		WillReturnRows(countRows)

	// SELECT query
	rows := sqlmock.NewRows(dongengColumns()).
		AddRow(id1, "Poor Pluto", 6, 9, "http://img/pluto.png", "", false, nil, int64(300), now, now, false).
		AddRow(id2, "Hansel & Gretel", 9, 12, "http://img/hansel.png", "", true, nil, int64(600), now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2 LIMIT $3`)).
		WithArgs(false, false, 10).
		WillReturnRows(rows)

	// id1 is not free, so its product mapping is resolved (no product -> unlocked).
	// id2 is free, so computeUnlocked short-circuits without querying.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_dongengs" WHERE dongeng_id = $1 ORDER BY "product_dongengs"."product_id" LIMIT $2`)).
		WithArgs(id1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "dongeng_id"}))

	result, err := svc.GetFairyTales("", 1, 10, nil, "", "")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Items, 2)

	assert.Equal(t, id1, result.Items[0].ID)
	assert.Equal(t, "Poor Pluto", result.Items[0].Title)
	assert.Equal(t, "5 min", result.Items[0].Duration)

	assert.Equal(t, id2, result.Items[1].ID)
	assert.Equal(t, "Hansel & Gretel", result.Items[1].Title)
	assert.Equal(t, "10 min", result.Items[1].Duration)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTales_EmptyDatabase(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2`)).
		WithArgs(false, false).
		WillReturnRows(countRows)

	rows := sqlmock.NewRows(dongengColumns())
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2 LIMIT $3`)).
		WithArgs(false, false, 10).
		WillReturnRows(rows)

	result, err := svc.GetFairyTales("", 1, 10, nil, "", "")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Items)
	assert.Equal(t, int64(0), result.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTales_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2`)).
		WithArgs(false, false).
		WillReturnError(sql.ErrConnDone)

	result, err := svc.GetFairyTales("", 1, 10, nil, "", "")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GetFairyTaleByID tests ───────────────────────────────────────────────────

func TestGetFairyTaleByID_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	dongengID := uuid.New()
	pageID := uuid.New()
	now := time.Now()

	dongengRows := sqlmock.NewRows(dongengColumns()).
		AddRow(dongengID, "Poor Pluto", 6, 9, "http://img/pluto.png", "", false, nil, int64(300), now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE id = $1 AND is_deleted = $2 AND hidden = $3 ORDER BY "dongengs"."id" LIMIT $4`)).
		WithArgs(dongengID.String(), false, false, 1).
		WillReturnRows(dongengRows)

	pageRows := sqlmock.NewRows([]string{
		"id", "dongeng_id", "page_number", "image_url", "text", "audio_url", "created_at", "updated_at", "is_deleted",
	}).AddRow(pageID, dongengID, 1, "http://img/page1.png", "Once upon a time...", "", now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongeng_pages" WHERE is_deleted = $1 AND "dongeng_pages"."dongeng_id" = $2 ORDER BY page_number ASC`)).
		WithArgs(false, dongengID).
		WillReturnRows(pageRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_dongengs" WHERE dongeng_id = $1 ORDER BY "product_dongengs"."product_id" LIMIT $2`)).
		WithArgs(dongengID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "dongeng_id"}))

	result, err := svc.GetFairyTaleByID(dongengID.String(), nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, dongengID, result.ID)
	assert.Equal(t, "Poor Pluto", result.Title)
	assert.Equal(t, "5 min", result.Duration)
	require.Len(t, result.Pages, 1)
	assert.Equal(t, 1, result.Pages[0].PageNumber)
	assert.Equal(t, "Once upon a time...", result.Pages[0].Text)
	assert.Nil(t, result.ProductID)
	assert.Nil(t, result.PriceIdr)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTaleByID_PaidContent_IncludesProductAndPrice(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	dongengID := uuid.New()
	productID := uuid.New()
	now := time.Now()

	dongengRows := sqlmock.NewRows(dongengColumns()).
		AddRow(dongengID, "Paket Berbayar", 6, 9, "http://img/x.png", "", false, nil, int64(300), now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE id = $1 AND is_deleted = $2 AND hidden = $3 ORDER BY "dongengs"."id" LIMIT $4`)).
		WithArgs(dongengID.String(), false, false, 1).
		WillReturnRows(dongengRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongeng_pages" WHERE is_deleted = $1 AND "dongeng_pages"."dongeng_id" = $2 ORDER BY page_number ASC`)).
		WithArgs(false, dongengID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "dongeng_id", "page_number", "image_url", "text", "audio_url", "created_at", "updated_at", "is_deleted"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_dongengs" WHERE dongeng_id = $1 ORDER BY "product_dongengs"."product_id" LIMIT $2`)).
		WithArgs(dongengID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "dongeng_id"}).AddRow(productID, dongengID))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(productID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}).
			AddRow(productID, uuid.New(), 9000, true, now, now))

	result, err := svc.GetFairyTaleByID(dongengID.String(), nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsUnlocked)
	require.NotNil(t, result.ProductID)
	assert.Equal(t, productID, *result.ProductID)
	require.NotNil(t, result.PriceIdr)
	assert.Equal(t, int64(9000), *result.PriceIdr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTaleByID_InactiveProduct_NoEntitlement_Hidden(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	dongengID := uuid.New()
	productID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	dongengRows := sqlmock.NewRows(dongengColumns()).
		AddRow(dongengID, "Paket Nonaktif", 6, 9, "http://img/x.png", "", false, nil, int64(300), now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE id = $1 AND is_deleted = $2 AND hidden = $3 ORDER BY "dongengs"."id" LIMIT $4`)).
		WithArgs(dongengID.String(), false, false, 1).
		WillReturnRows(dongengRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongeng_pages" WHERE is_deleted = $1 AND "dongeng_pages"."dongeng_id" = $2 ORDER BY page_number ASC`)).
		WithArgs(false, dongengID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "dongeng_id", "page_number", "image_url", "text", "audio_url", "created_at", "updated_at", "is_deleted"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_dongengs" WHERE dongeng_id = $1 ORDER BY "product_dongengs"."product_id" LIMIT $2`)).
		WithArgs(dongengID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "dongeng_id"}).AddRow(productID, dongengID))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(productID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}).
			AddRow(productID, uuid.New(), 9000, false, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "user_entitlements" WHERE user_id = $1 AND product_id = $2 AND (expires_at IS NULL OR expires_at > NOW())`)).
		WithArgs(userID, productID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	result, err := svc.GetFairyTaleByID(dongengID.String(), &userID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsUnlocked)
	assert.Nil(t, result.ProductID)
	assert.Nil(t, result.PriceIdr)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTaleByID_InactiveProduct_ExistingOwner_StillUnlocked(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	dongengID := uuid.New()
	productID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	dongengRows := sqlmock.NewRows(dongengColumns()).
		AddRow(dongengID, "Paket Nonaktif", 6, 9, "http://img/x.png", "", false, nil, int64(300), now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE id = $1 AND is_deleted = $2 AND hidden = $3 ORDER BY "dongengs"."id" LIMIT $4`)).
		WithArgs(dongengID.String(), false, false, 1).
		WillReturnRows(dongengRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongeng_pages" WHERE is_deleted = $1 AND "dongeng_pages"."dongeng_id" = $2 ORDER BY page_number ASC`)).
		WithArgs(false, dongengID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "dongeng_id", "page_number", "image_url", "text", "audio_url", "created_at", "updated_at", "is_deleted"}))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "product_dongengs" WHERE dongeng_id = $1 ORDER BY "product_dongengs"."product_id" LIMIT $2`)).
		WithArgs(dongengID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"product_id", "dongeng_id"}).AddRow(productID, dongengID))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1 ORDER BY "products"."id" LIMIT $2`)).
		WithArgs(productID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature_id", "price_idr", "is_active", "created_at", "updated_at"}).
			AddRow(productID, uuid.New(), 9000, false, now, now))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_subscriptions" WHERE user_id = $1 ORDER BY "user_subscriptions"."id" LIMIT $2`)).
		WithArgs(userID, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "user_entitlements" WHERE user_id = $1 AND product_id = $2 AND (expires_at IS NULL OR expires_at > NOW())`)).
		WithArgs(userID, productID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	result, err := svc.GetFairyTaleByID(dongengID.String(), &userID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsUnlocked)
	require.NotNil(t, result.ProductID)
	assert.Equal(t, productID, *result.ProductID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── RecordPlay / UpdateProgress / GetHistory tests ──────────────────────────

func TestDongengService_RecordPlay_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	userID := uuid.New()
	dongengID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "dongeng_play_history"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	err := svc.RecordPlay(userID, dongengID)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDongengService_UpdateProgress_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	userID := uuid.New()
	dongengID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "dongeng_play_history"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	err := svc.UpdateProgress(userID, dongengID, 42)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDongengService_GetHistory_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	userID := uuid.New()
	dongengID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT h.dongeng_id, h.progress_seconds, d.duration AS total_seconds, h.started_at FROM dongeng_play_history h JOIN dongengs d ON d.id = h.dongeng_id AND d.is_deleted = false AND d.hidden = false WHERE h.user_id = $1 ORDER BY h.updated_at DESC LIMIT $2`)).
		WithArgs(userID, 20).
		WillReturnRows(sqlmock.NewRows([]string{"dongeng_id", "progress_seconds", "total_seconds", "started_at"}).
			AddRow(dongengID, 30, int64(300), now))

	items, err := svc.GetHistory(userID)

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, dongengID, items[0].DongengID)
	assert.Equal(t, 30, items[0].ProgressSeconds)
	assert.Equal(t, int64(300), items[0].TotalSeconds)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GetCategories tests ──────────────────────────────────────────────────────

func TestDongengService_GetCategories_Empty(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	mock.ExpectQuery(`SELECT \* FROM "dongeng_categories" WHERE parent_id IS NULL AND is_deleted = \$1 ORDER BY sort_order ASC`).
		WithArgs(false).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "image_url", "parent_id", "sort_order", "created_at", "updated_at", "is_deleted"}))

	cats, err := svc.GetCategories()

	require.NoError(t, err)
	assert.Empty(t, cats)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDongengService_GetCategories_WithTopLevelCategories(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	id1 := uuid.New()
	id2 := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`SELECT \* FROM "dongeng_categories" WHERE parent_id IS NULL AND is_deleted = \$1 ORDER BY sort_order ASC`).
		WithArgs(false).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "image_url", "parent_id", "sort_order", "created_at", "updated_at", "is_deleted"}).
			AddRow(id1, "Fairy Tales", "", nil, 0, now, now, false).
			AddRow(id2, "Islamic", "", nil, 1, now, now, false))

	mock.ExpectQuery(`SELECT \* FROM "dongeng_categories" WHERE .*"dongeng_categories"\."parent_id" IN \(\$1,\$2\).*is_deleted = \$3`).
		WithArgs(id1, id2, false).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "image_url", "parent_id", "sort_order", "created_at", "updated_at", "is_deleted"}))

	cats, err := svc.GetCategories()

	require.NoError(t, err)
	require.Len(t, cats, 2)
	assert.Equal(t, "Fairy Tales", cats[0].Name)
	assert.Equal(t, "Islamic", cats[1].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── Category filtering tests ─────────────────────────────────────────────────

func TestGetFairyTales_FilterByCategory(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	categoryID := uuid.New().String()
	id1 := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2 AND dongeng_category_id = $3`)).
		WithArgs(false, false, categoryID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE is_deleted = $1 AND hidden = $2 AND dongeng_category_id = $3 LIMIT $4`)).
		WithArgs(false, false, categoryID, 10).
		WillReturnRows(sqlmock.NewRows(dongengColumns()).
			AddRow(id1, "Poor Pluto", 6, 9, "http://img/pluto.png", "", true, nil, int64(300), now, now, false))

	result, err := svc.GetFairyTales("", 1, 10, nil, categoryID, "")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "Poor Pluto", result.Items[0].Title)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTaleByID_NotFound(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE id = $1 AND is_deleted = $2 AND hidden = $3 ORDER BY "dongengs"."id" LIMIT $4`)).
		WithArgs("non-existent-id", false, false, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	result, err := svc.GetFairyTaleByID("non-existent-id", nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFairyTaleByID_IncludesCategoryRef(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := newDongengService(gormDB)

	dongengID := uuid.New()
	categoryID := uuid.New()
	now := time.Now()

	dongengCols := append(dongengColumns(), "dongeng_category_id", "dongeng_sub_category_id")
	dongengRows := sqlmock.NewRows(dongengCols).
		AddRow(dongengID, "Poor Pluto", 6, 9, "http://img/pluto.png", "", true, nil, int64(300), now, now, false, categoryID, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE id = $1 AND is_deleted = $2 AND hidden = $3 ORDER BY "dongengs"."id" LIMIT $4`)).
		WithArgs(dongengID.String(), false, false, 1).
		WillReturnRows(dongengRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongeng_categories" WHERE "dongeng_categories"."id" = $1`)).
		WithArgs(categoryID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "image_url", "parent_id", "sort_order", "created_at", "updated_at", "is_deleted"}).
			AddRow(categoryID, "Fairy Tales", "", nil, 0, now, now, false))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongeng_pages" WHERE is_deleted = $1 AND "dongeng_pages"."dongeng_id" = $2 ORDER BY page_number ASC`)).
		WithArgs(false, dongengID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "dongeng_id", "page_number", "image_url", "text", "audio_url", "created_at", "updated_at", "is_deleted"}))

	result, err := svc.GetFairyTaleByID(dongengID.String(), nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.CategoryRef)
	assert.Equal(t, "Fairy Tales", result.CategoryRef.Name)
	assert.Nil(t, result.SubCategoryRef)
	assert.NoError(t, mock.ExpectationsWereMet())
}
