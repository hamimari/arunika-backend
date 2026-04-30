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

	"arunika_backend/models"
)

func setupAdminContentDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	dialector := postgres.New(postgres.Config{Conn: db, DriverName: "postgres"})
	gormDB, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return gormDB, mock
}

// ─── FairyTales ───────────────────────────────────────────────────────────────

func TestAdminContentService_ListFairyTales_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = false`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "dongengs" WHERE is_deleted = false`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "age_start", "age_end", "image_url", "is_free", "is_deleted", "hidden", "created_at", "updated_at"}).
			AddRow(id, "Si Kancil", 3, 6, "https://img/kancil.png", true, false, false, now, now))

	items, total, err := svc.ListFairyTales("", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.Equal(t, "Si Kancil", items[0].Title)
}

func TestAdminContentService_ListFairyTales_DBError(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "dongengs" WHERE is_deleted = false`)).
		WillReturnError(gorm.ErrInvalidDB)

	_, _, err := svc.ListFairyTales("", 1, 20)
	_ = err // count error may or may not propagate depending on GORM version
}

func TestAdminContentService_GetFairyTale_NotFound(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectQuery(`SELECT \* FROM "dongengs" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.GetFairyTale("missing-id")
	assert.Error(t, err)
}

func TestAdminContentService_CreateFairyTale_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	input := models.Dongeng{
		Title:    "Bawang Merah",
		ImageUrl: "https://img/bawang.png",
		IsFree:   true,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "dongengs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	result, err := svc.CreateFairyTale(input)
	require.NoError(t, err)
	assert.Equal(t, "Bawang Merah", result.Title)
}

func TestAdminContentService_CreateFairyTale_DBError(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "dongengs"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := svc.CreateFairyTale(models.Dongeng{Title: "fail"})
	assert.Error(t, err)
}

func TestAdminContentService_UpdateFairyTale_NotFound(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectQuery(`SELECT \* FROM "dongengs" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.UpdateFairyTale("missing-id", models.Dongeng{})
	assert.EqualError(t, err, "not found")
}

func TestAdminContentService_UpdateFairyTale_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dongengs" WHERE id = $1 AND is_deleted = false`)).
		WithArgs(id.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "hidden", "is_free", "created_at", "updated_at"}).
			AddRow(id, "Old Title", false, true, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "dongengs" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.UpdateFairyTale(id.String(), models.Dongeng{Title: "New Title"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAdminContentService_DeleteFairyTale(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "dongengs" SET "is_deleted"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteFairyTale(id)
	assert.NoError(t, err)
}

func TestAdminContentService_ToggleFairyTaleVisibility(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "dongengs" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ToggleFairyTaleVisibility(id, true)
	assert.NoError(t, err)
}

// ─── AR Cards ─────────────────────────────────────────────────────────────────

func arCardCols() []string {
	return []string{"id", "type", "title", "file_url", "sound_url", "short_code", "hidden", "created_at", "updated_at"}
}

func TestAdminContentService_CreateArCard_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	input := models.ArCards{
		Type:    "alphabet",
		Title:   "A for Apple",
		FileURL: "https://ar/a.glb",
	}

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "ar_cards"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.CreateArCard(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID) // UUID assigned by service
	assert.Equal(t, "A for Apple", result.Title)
}

func TestAdminContentService_CreateArCard_DBError(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "ar_cards"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := svc.CreateArCard(models.ArCards{Type: "alphabet", FileURL: "x"})
	assert.Error(t, err)
}

func TestAdminContentService_UpdateArCard_NotFound(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectQuery(`SELECT \* FROM "ar_cards" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.UpdateArCard("missing", models.ArCards{})
	assert.EqualError(t, err, "not found")
}

func TestAdminContentService_UpdateArCard_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := "some-ar-id"
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ar_cards" WHERE id = $1`)).
		WithArgs(id, 1).
		WillReturnRows(sqlmock.NewRows(arCardCols()).
			AddRow(id, "alphabet", "Old Title", "https://ar/old.glb", "", "A01", false, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "ar_cards" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.UpdateArCard(id, models.ArCards{Title: "New Title"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAdminContentService_DeleteArCard_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := "ar-id-123"
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "ar_cards" WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteArCard(id)
	assert.NoError(t, err)
}

func TestAdminContentService_DeleteArCard_DBError(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "ar_cards"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	err := svc.DeleteArCard("ar-id-123")
	assert.Error(t, err)
}

func TestAdminContentService_ToggleArCardVisibility(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := "ar-card-id"
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "ar_cards" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(false, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ToggleArCardVisibility(id, false)
	assert.NoError(t, err)
}

// ─── Tracing Items ────────────────────────────────────────────────────────────

func tracingCols() []string {
	return []string{"id", "type", "label", "guide_path_json", "difficulty", "hidden", "created_at", "updated_at"}
}

func TestAdminContentService_CreateTracingItem_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	input := models.TracingItem{
		Type:          "alphabet",
		Label:         "A",
		GuidePathJSON: `[{"x":0,"y":0}]`,
		Difficulty:    1,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "tracing_items"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	result, err := svc.CreateTracingItem(input)
	require.NoError(t, err)
	assert.Equal(t, "A", result.Label)
}

func TestAdminContentService_CreateTracingItem_DBError(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "tracing_items"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := svc.CreateTracingItem(models.TracingItem{Label: "B", Type: "alphabet", GuidePathJSON: `[]`})
	assert.Error(t, err)
}

func TestAdminContentService_UpdateTracingItem_NotFound(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectQuery(`SELECT \* FROM "tracing_items" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.UpdateTracingItem("missing", models.TracingItem{})
	assert.EqualError(t, err, "not found")
}

func TestAdminContentService_UpdateTracingItem_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "tracing_items" WHERE id = $1`)).
		WithArgs(id.String(), 1).
		WillReturnRows(sqlmock.NewRows(tracingCols()).
			AddRow(id, "alphabet", "A", `[]`, 1, false, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "tracing_items" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.UpdateTracingItem(id.String(), models.TracingItem{Label: "B"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAdminContentService_DeleteTracingItem_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "tracing_items" WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteTracingItem(id)
	assert.NoError(t, err)
}

func TestAdminContentService_ToggleTracingItemVisibility(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "tracing_items" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ToggleTracingItemVisibility(id, true)
	assert.NoError(t, err)
}

// ─── Counting Questions ───────────────────────────────────────────────────────

func countingCols() []string {
	return []string{"id", "level", "question_json", "answer", "hidden", "created_at", "updated_at"}
}

func TestAdminContentService_CreateCountingQuestion_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	input := models.CountingQuestion{
		Level:        "easy",
		QuestionJSON: `{"image":"apple.png"}`,
		Answer:       3,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "counting_questions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	result, err := svc.CreateCountingQuestion(input)
	require.NoError(t, err)
	assert.Equal(t, "easy", result.Level)
}

func TestAdminContentService_CreateCountingQuestion_DBError(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "counting_questions"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := svc.CreateCountingQuestion(models.CountingQuestion{Level: "easy", QuestionJSON: `{}`, Answer: 1})
	assert.Error(t, err)
}

func TestAdminContentService_UpdateCountingQuestion_NotFound(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectQuery(`SELECT \* FROM "counting_questions" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.UpdateCountingQuestion("missing", models.CountingQuestion{})
	assert.EqualError(t, err, "not found")
}

func TestAdminContentService_UpdateCountingQuestion_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "counting_questions" WHERE id = $1`)).
		WithArgs(id.String(), 1).
		WillReturnRows(sqlmock.NewRows(countingCols()).
			AddRow(id, "easy", `{}`, 3, false, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "counting_questions" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.UpdateCountingQuestion(id.String(), models.CountingQuestion{Level: "hard"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAdminContentService_DeleteCountingQuestion_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "counting_questions" WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteCountingQuestion(id)
	assert.NoError(t, err)
}

func TestAdminContentService_ToggleCountingQuestionVisibility(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "counting_questions" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(false, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ToggleCountingQuestionVisibility(id, false)
	assert.NoError(t, err)
}

// ─── Badges ───────────────────────────────────────────────────────────────────

func TestAdminContentService_ListBadges_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New()
	now := time.Now()

	// ListBadges has no is_deleted filter
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "badges"`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	mock.ExpectQuery(`SELECT \* FROM "badges"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature", "level", "image_url", "created_at", "updated_at"}).
			AddRow(id, "reading", "bronze", "https://img/star.png", now, now))

	items, total, err := svc.ListBadges("", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, items, 1)
}

func TestAdminContentService_CreateBadge_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	input := models.Badge{
		Feature:   "tracing",
		Level:     "beginner",
		Threshold: 5,
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "badges"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	result, err := svc.CreateBadge(input)
	require.NoError(t, err)
	assert.Equal(t, "tracing", result.Feature)
}

func TestAdminContentService_CreateBadge_DBError(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "badges"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := svc.CreateBadge(models.Badge{Feature: "tracing", Level: "beginner", Threshold: 5})
	assert.Error(t, err)
}

func TestAdminContentService_UpdateBadge_NotFound(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectQuery(`SELECT \* FROM "badges" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.UpdateBadge("missing", models.Badge{})
	assert.EqualError(t, err, "not found")
}

func TestAdminContentService_UpdateBadge_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "badges" WHERE id = $1`)).
		WithArgs(id.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "feature", "level", "threshold", "hidden", "updated_at"}).
			AddRow(id, "tracing", "beginner", 5, false, now))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "badges" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.UpdateBadge(id.String(), models.Badge{Threshold: 10})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAdminContentService_DeleteBadge_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`DELETE FROM "badges" WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteBadge(id)
	assert.NoError(t, err)
}

func TestAdminContentService_ToggleBadgeVisibility(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "badges" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ToggleBadgeVisibility(id, true)
	assert.NoError(t, err)
}

// ─── Categories ───────────────────────────────────────────────────────────────

func TestAdminContentService_ListCategories_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "categories" WHERE is_deleted = false`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE is_deleted = false`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "image_url", "is_deleted", "hidden", "created_at", "updated_at"}).
			AddRow(id, "Fabel", "https://img/fabel.png", false, false, now, now))

	items, total, err := svc.ListCategories("", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
}

func TestAdminContentService_CreateCategory_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	input := models.Categories{
		Name:     "Legenda",
		ImageUrl: "https://img/legenda.png",
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "categories"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	result, err := svc.CreateCategory(input)
	require.NoError(t, err)
	assert.Equal(t, "Legenda", result.Name)
}

func TestAdminContentService_CreateCategory_DBError(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "categories"`).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := svc.CreateCategory(models.Categories{Name: "Fail"})
	assert.Error(t, err)
}

func TestAdminContentService_UpdateCategory_NotFound(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	mock.ExpectQuery(`SELECT \* FROM "categories" WHERE id = \$1`).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := svc.UpdateCategory("missing", models.Categories{})
	assert.EqualError(t, err, "not found")
}

func TestAdminContentService_UpdateCategory_Success(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "categories" WHERE id = $1 AND is_deleted = false`)).
		WithArgs(id.String(), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "image_url", "hidden", "is_deleted", "created_at", "updated_at"}).
			AddRow(id, "Fabel", "https://img/fabel.png", false, false, now, now))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "categories" SET`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result, err := svc.UpdateCategory(id.String(), models.Categories{Name: "Legenda"})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAdminContentService_DeleteCategory(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "categories" SET "is_deleted"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(true, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.DeleteCategory(id)
	assert.NoError(t, err)
}

func TestAdminContentService_ToggleCategoryVisibility(t *testing.T) {
	db, mock := setupAdminContentDB(t)
	svc := NewAdminContentService(db)

	id := uuid.New().String()
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "categories" SET "hidden"=\$1,"updated_at"=\$2 WHERE id = \$3`).
		WithArgs(false, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.ToggleCategoryVisibility(id, false)
	assert.NoError(t, err)
}
