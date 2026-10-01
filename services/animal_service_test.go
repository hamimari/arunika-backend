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

func animalColumns() []string {
	return []string{
		"id", "name", "emoji", "category", "image_url", "bg_color",
		"fact", "is_unlocked", "created_at", "updated_at", "is_deleted",
	}
}

func TestAnimalService_GetAnimals_Success(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAnimalService(gormDB)

	id1 := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows(animalColumns()).
		AddRow(id1, "Sapi", "🐄", "ternak", "https://cdn/sapi.png", "#FFF3E0",
			"Sapi punya empat lambung.", false, now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "animals" WHERE is_deleted = false ORDER BY name asc`,
	)).WillReturnRows(rows)

	animals, err := svc.GetAnimals("")

	require.NoError(t, err)
	assert.Len(t, animals, 1)
	assert.Equal(t, "Sapi", animals[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnimalService_GetAnimals_WithCategory(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAnimalService(gormDB)

	id1 := uuid.New()
	now := time.Now()

	rows := sqlmock.NewRows(animalColumns()).
		AddRow(id1, "Harimau", "🐯", "hutan", "https://cdn/harimau.png", "#FFF3E0",
			"Harimau adalah karnivora.", false, now, now, false)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "animals" WHERE is_deleted = false AND category = $1 ORDER BY name asc`,
	)).WithArgs("hutan").WillReturnRows(rows)

	animals, err := svc.GetAnimals("hutan")

	require.NoError(t, err)
	assert.Len(t, animals, 1)
	assert.Equal(t, "hutan", animals[0].Category)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAnimalService_GetAnimals_DBError(t *testing.T) {
	gormDB, mock := setupMockDB(t)
	svc := NewAnimalService(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "animals" WHERE is_deleted = false ORDER BY name asc`,
	)).WillReturnError(gorm.ErrInvalidDB)

	animals, err := svc.GetAnimals("")

	assert.Error(t, err)
	assert.Nil(t, animals)
	assert.NoError(t, mock.ExpectationsWereMet())
}
