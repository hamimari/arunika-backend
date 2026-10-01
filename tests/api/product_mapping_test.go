package api_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// A single AR card / dongeng can only be bought through Google Play once an
// admin maps its product to a Play SKU.

func playSKUOf(t *testing.T, env *Env, id string) *string {
	t.Helper()
	var p models.Product
	require.NoError(t, env.DB.First(&p, "id = ?", id).Error)
	return p.PlayProductID
}

func TestAdmin_MapsAProductToAPlaySKU_AndItBecomesPurchasable(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	account := env.Register(t)
	product := fixtures.NewProduct(t, env.DB)
	id := product.ID.String()

	// Unmapped: the app can't buy it through Google Play.
	before := env.POST("/payment/play/create-product", account.Token, map[string]string{"product_id": id})
	assert.Equal(t, http.StatusBadRequest, before.Code, string(before.Body))

	res := env.PUT("/admin/products/"+id, admin, map[string]interface{}{
		"price_idr": 25000, "play_product_id": "  sku_frog  ",
	})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	require.NotNil(t, playSKUOf(t, env, id))
	assert.Equal(t, "sku_frog", *playSKUOf(t, env, id), "the SKU is trimmed")

	list := env.GET("/admin/products", admin)
	require.Equal(t, http.StatusOK, list.Code)
	var listed interface{}
	for _, it := range list.JSON()["data"].([]interface{}) {
		if it.(map[string]interface{})["id"] == id {
			listed = it.(map[string]interface{})["play_product_id"]
		}
	}
	assert.Equal(t, "sku_frog", listed, "the admin list shows the mapping")

	after := env.POST("/payment/play/create-product", account.Token, map[string]string{"product_id": id})
	require.Equal(t, http.StatusOK, after.Code, string(after.Body))
	assert.Equal(t, "sku_frog", after.Data()["play_product_id"])
}

func TestAdmin_ProductUpdate_KeepsTheMappingUnlessTold(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	product := fixtures.NewProduct(t, env.DB, fixtures.WithPlayProductID("sku_keep"))
	id := product.ID.String()

	// A client that doesn't know about the field just edits the price.
	res := env.PUT("/admin/products/"+id, admin, map[string]interface{}{"price_idr": 30000})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	require.NotNil(t, playSKUOf(t, env, id))
	assert.Equal(t, "sku_keep", *playSKUOf(t, env, id), "an absent field leaves the mapping alone")

	res = env.PUT("/admin/products/"+id, admin, map[string]interface{}{"price_idr": 30000, "play_product_id": nil})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	assert.Nil(t, playSKUOf(t, env, id), "null clears it")

	require.NoError(t, env.DB.Exec(`UPDATE products SET play_product_id = 'sku_keep' WHERE id = ?`, id).Error)
	res = env.PUT("/admin/products/"+id, admin, map[string]interface{}{"price_idr": 30000, "play_product_id": "   "})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	assert.Nil(t, playSKUOf(t, env, id), "a blank SKU clears it")
}

func TestAdmin_RejectsASKUAlreadyUsedElsewhere(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	fixtures.NewProduct(t, env.DB, fixtures.WithPlayProductID("sku_taken"))
	fixtures.NewPackage(t, env.DB, fixtures.WithPackagePlayProductID("pkg_taken"))
	other := fixtures.NewProduct(t, env.DB)

	for _, sku := range []string{"sku_taken", "pkg_taken"} {
		res := env.PUT("/admin/products/"+other.ID.String(), admin,
			map[string]interface{}{"price_idr": 25000, "play_product_id": sku})
		assert.Equal(t, http.StatusBadRequest, res.Code, "%s: %s", sku, string(res.Body))
		assert.Contains(t, string(res.Body), "already used")
	}
	assert.Nil(t, playSKUOf(t, env, other.ID.String()))

	// Saving a product with its own SKU again is fine.
	own := fixtures.NewProduct(t, env.DB, fixtures.WithPlayProductID("sku_own"))
	res := env.PUT("/admin/products/"+own.ID.String(), admin,
		map[string]interface{}{"price_idr": 25000, "play_product_id": "sku_own"})
	assert.Equal(t, http.StatusOK, res.Code, string(res.Body))

	bad := env.PUT("/admin/products/"+own.ID.String(), admin,
		map[string]interface{}{"price_idr": 25000, "play_product_id": 42})
	assert.Equal(t, http.StatusBadRequest, bad.Code, "a non-string SKU is rejected")
}

func TestAdmin_CreatesAProductWithASKU(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	dongeng := fixtures.NewDongeng(t, env.DB, fixtures.Paid())

	res := env.POST("/admin/products", admin, map[string]interface{}{
		"feature_code": "DONGENG", "price_idr": 39000,
		"dongeng_id": dongeng.ID.String(), "play_product_id": "sku_new_tale",
	})
	require.Equal(t, http.StatusCreated, res.Code, string(res.Body))
	assert.Equal(t, "sku_new_tale", res.Data()["play_product_id"])

	dup := env.POST("/admin/products", admin, map[string]interface{}{
		"feature_code": "DONGENG", "price_idr": 39000,
		"dongeng_id":      fixtures.NewDongeng(t, env.DB, fixtures.Paid()).ID.String(),
		"play_product_id": "sku_new_tale",
	})
	assert.Equal(t, http.StatusBadRequest, dup.Code, string(dup.Body))
}
