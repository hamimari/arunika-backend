package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"arunika_backend/services"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strikeRuleColumns() []string {
	return []string{"scope", "mode", "value", "starts_at", "ends_at", "updated_at"}
}

func putJSON(t *testing.T, path string, params gin.Params, body interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, path, bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	return c, w
}

func TestStrikePriceHandler_AdminList(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewStrikePriceHandler(services.NewStrikePriceService(gormDB))
	end := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "strike_price_rules" ORDER BY scope ASC`)).
		WillReturnRows(sqlmock.NewRows(strikeRuleColumns()).
			AddRow("AR_CARD", "NONE", 0, nil, nil, time.Now()).
			AddRow("PACKAGE", "PERCENT", 20, nil, end, time.Now()))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/strike-price-rules", nil)
	h.AdminList(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data []struct {
			Scope  string `json:"scope"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)
	assert.Equal(t, "OFF", resp.Data[0].Status)
	assert.Equal(t, "ACTIVE", resp.Data[1].Status)
}

func TestStrikePriceHandler_AdminUpdate_Success(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewStrikePriceHandler(services.NewStrikePriceService(gormDB))
	end := time.Now().AddDate(0, 0, 30).UTC().Truncate(time.Second)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "strike_price_rules" SET "ends_at"=$1,"mode"=$2,"starts_at"=$3,"updated_at"=$4,"value"=$5 WHERE scope = $6`)).
		WithArgs(sqlmock.AnyArg(), "PERCENT", nil, sqlmock.AnyArg(), 20, "AR_CARD").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "strike_price_rules" WHERE scope = $1`)).
		WillReturnRows(sqlmock.NewRows(strikeRuleColumns()).AddRow("AR_CARD", "PERCENT", 20, nil, end, time.Now()))

	c, w := putJSON(t, "/admin/strike-price-rules/AR_CARD", gin.Params{{Key: "scope", Value: "AR_CARD"}},
		map[string]interface{}{"mode": "PERCENT", "value": 20, "ends_at": end})
	h.AdminUpdate(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStrikePriceHandler_AdminUpdate_UnknownScope(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	h := NewStrikePriceHandler(services.NewStrikePriceService(gormDB))

	c, w := putJSON(t, "/admin/strike-price-rules/BUNDLE", gin.Params{{Key: "scope", Value: "BUNDLE"}},
		map[string]interface{}{"mode": "NONE"})
	h.AdminUpdate(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestStrikePriceHandler_AdminUpdate_OutOfRangePercent(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewStrikePriceHandler(services.NewStrikePriceService(gormDB))

	c, w := putJSON(t, "/admin/strike-price-rules/PACKAGE", gin.Params{{Key: "scope", Value: "PACKAGE"}},
		map[string]interface{}{"mode": "PERCENT", "value": 95, "ends_at": time.Now().AddDate(0, 0, 10)})
	h.AdminUpdate(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet(), "nothing written")
}

func TestStrikePriceHandler_AdminUpdate_TooLongPromo(t *testing.T) {
	gormDB, _ := setupHandlerDB(t)
	h := NewStrikePriceHandler(services.NewStrikePriceService(gormDB))

	c, w := putJSON(t, "/admin/strike-price-rules/DONGENG", gin.Params{{Key: "scope", Value: "DONGENG"}},
		map[string]interface{}{"mode": "FIXED", "value": 10000, "ends_at": time.Now().AddDate(0, 0, 120)})
	h.AdminUpdate(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminProductHandler_Update_StrikeWithoutEndDate(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewAdminProductHandler(services.NewProductService(gormDB))
	id := uuid.New()

	c, w := putJSON(t, "/admin/products/"+id.String(), gin.Params{{Key: "id", Value: id.String()}},
		map[string]interface{}{"price_idr": 15000, "strike_mode": "FIXED", "strike_value": 5000})
	h.Update(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet(), "nothing written")
}

func TestAdminProductHandler_Update_WithStrikeOverride(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewAdminProductHandler(services.NewProductService(gormDB))
	id := uuid.New()
	end := time.Now().AddDate(0, 0, 14).UTC().Truncate(time.Second)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "products" SET "price_idr"=$1,"strike_ends_at"=$2,"strike_mode"=$3,"strike_starts_at"=$4,"strike_value"=$5,"updated_at"=$6 WHERE id = $7`)).
		WithArgs(int64(15000), sqlmock.AnyArg(), "FIXED", nil, 5000, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "products" WHERE id = $1`)).
		WillReturnRows(sqlmock.NewRows(productColumns()).AddRow(id, uuid.New(), 15000, true, time.Now(), time.Now()))

	c, w := putJSON(t, "/admin/products/"+id.String(), gin.Params{{Key: "id", Value: id.String()}},
		map[string]interface{}{"price_idr": 15000, "strike_mode": "FIXED", "strike_value": 5000, "strike_ends_at": end})
	h.Update(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPremiumPackHandler_AdminUpdatePack_InvalidStrike(t *testing.T) {
	gormDB, mock := setupHandlerDB(t)
	h := NewPremiumPackHandler(services.NewPremiumPackService(gormDB, services.NewOrderService(gormDB, services.NewProductService(gormDB))))

	c, w := putJSON(t, "/admin/premium/packs/p1", gin.Params{{Key: "id", Value: "p1"}},
		map[string]interface{}{
			"name": "Paket Hutan", "subtitle": "8 hewan", "price_idr": 79000, "type": "content",
			"strike_mode": "PERCENT", "strike_value": 20, "strike_ends_at": time.Now().Add(-time.Hour),
		})
	h.AdminUpdatePack(c)

	assert.Equal(t, http.StatusBadRequest, w.Code, "an end date in the past is rejected")
	assert.NoError(t, mock.ExpectationsWereMet(), "nothing written")
}
