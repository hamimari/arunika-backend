package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"arunika_backend/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Edit forms in the backoffice never send `hidden` — visibility is owned by
// the PATCH …/visibility toggle — so saving an edit must not reset it.
func TestAdminContentUpdates_DoNotOverwriteHidden(t *testing.T) {
	var updates []string
	matcher := sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if strings.HasPrefix(actual, "UPDATE") {
			updates = append(updates, actual)
		}
		if !strings.HasPrefix(actual, expected) {
			return fmt.Errorf("unexpected sql %q", actual)
		}
		return nil
	})
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	svc := NewAdminContentService(db)
	now := time.Now()

	dongengID := uuid.New()
	mock.ExpectQuery(`SELECT * FROM "dongengs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "hidden", "created_at", "updated_at"}).AddRow(dongengID, "Old", true, now, now))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "dongengs" SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	_, err = svc.UpdateFairyTale(dongengID.String(), models.Dongeng{Title: "New"})
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT * FROM "ar_cards"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "hidden", "created_at", "updated_at"}).AddRow("card-1", "Old", true, now, now))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "ar_cards" SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	_, err = svc.UpdateArCard("card-1", models.ArCards{Title: "New"})
	require.NoError(t, err)

	require.Len(t, updates, 2)
	for _, sql := range updates {
		assert.NotContains(t, sql, `"hidden"`, sql)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}
