package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func roleRouter(t *testing.T, role string, dbErr error) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)

	q := mock.ExpectQuery(`SELECT "role" FROM "admin_users"`)
	if dbErr != nil {
		q.WillReturnError(dbErr)
	} else {
		rows := sqlmock.NewRows([]string{"role"})
		if role != "" {
			rows.AddRow(role)
		}
		q.WillReturnRows(rows)
	}

	r := gin.New()
	r.POST("/x", func(c *gin.Context) { c.Set("adminID", "a1"); c.Next() },
		RequireAdminRole(db, "publisher"),
		func(c *gin.Context) { c.String(http.StatusOK, c.GetString("adminRole")) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))
	return w
}

func TestRequireAdminRole(t *testing.T) {
	ok := roleRouter(t, "publisher", nil)
	assert.Equal(t, http.StatusOK, ok.Code)
	assert.Equal(t, "publisher", ok.Body.String())

	assert.Equal(t, http.StatusForbidden, roleRouter(t, "editor", nil).Code)
	assert.Equal(t, http.StatusForbidden, roleRouter(t, "", nil).Code, "deleted or unknown admin")
	assert.Equal(t, http.StatusForbidden, roleRouter(t, "", errors.New("db down")).Code)
}
