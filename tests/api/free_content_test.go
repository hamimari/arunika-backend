package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// An admin can make paid content free and back without touching its product,
// orders or the entitlements of people who already bought it.

func newArCard(t *testing.T, env *Env, isFree bool) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, env.DB.Create(&models.ArCards{
		ID: id, Type: "alphabet", Title: "Card " + id[:6], FileURL: "https://cdn/x.glb",
		ShortCode: fmt.Sprintf("SC%d", fixtures.NextSeq()), IsFree: isFree,
	}).Error)
	// GORM skips a false bool on create when the column has a default.
	require.NoError(t, env.DB.Model(&models.ArCards{}).Where("id = ?", id).Update("is_free", isFree).Error)
	return id
}

// soldArCard is an AR card sold as a product, with a Play SKU.
func soldArCard(t *testing.T, env *Env, opts ...fixtures.ProductOption) (string, *models.Product) {
	t.Helper()
	id := newArCard(t, env, false)
	opts = append([]fixtures.ProductOption{fixtures.WithPlayProductID(fmt.Sprintf("sku_%d", fixtures.NextSeq()))}, opts...)
	product := fixtures.NewProduct(t, env.DB, opts...)
	require.NoError(t, env.DB.Create(&models.ProductArCard{ProductID: product.ID, ArCardID: id}).Error)
	return id, product
}

func setFree(env *Env, kind, admin, id string, isFree bool) *Response {
	return env.do(http.MethodPatch, "/admin/content/"+kind+"/"+id+"/free", admin, map[string]interface{}{"is_free": isFree})
}

func publicCard(t *testing.T, env *Env, token, id string) map[string]interface{} {
	t.Helper()
	res := env.GET("/ar/cards/"+id, token)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	return res.JSON() // the public card endpoint is not wrapped in {"data": ...}
}

func adminAccess(t *testing.T, env *Env, admin, kind, id string) (string, interface{}) {
	t.Helper()
	res := env.GET("/admin/content/"+kind+"/"+id, admin)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	d := res.Data()
	return d["access"].(string), d["price_idr"]
}

func TestFreeContent_MakingAPaidCardFree_UnlocksItForEveryone_AndBackAgain(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	stranger := env.Register(t)
	owner := env.Register(t)
	cardID, product := soldArCard(t, env, fixtures.WithPrice(15_000))

	var ownerUser models.Parent
	require.NoError(t, env.DB.First(&ownerUser, "id = ?", owner.ID).Error)
	fixtures.NewEntitlement(t, env.DB, &ownerUser, product)

	before := publicCard(t, env, stranger.Token, cardID)
	assert.Equal(t, false, before["is_unlocked"])
	assert.NotNil(t, before["product_id"])
	access, price := adminAccess(t, env, admin, "ar-cards", cardID)
	assert.Equal(t, "PAID", access)
	assert.EqualValues(t, 15000, price)

	res := setFree(env, "ar-cards", admin, cardID, true)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))

	for _, token := range []string{stranger.Token, ""} { // signed in, and a guest
		card := publicCard(t, env, token, cardID)
		assert.Equal(t, true, card["is_unlocked"])
		assert.Nil(t, card["product_id"], "a free card offers nothing to buy")
		assert.Nil(t, card["price_idr"])
	}
	access, price = adminAccess(t, env, admin, "ar-cards", cardID)
	assert.Equal(t, "FREE", access)
	assert.EqualValues(t, 15000, price, "the product's price is still shown to the admin")

	// The product, and the buyer's entitlement, were not touched.
	var products, entitlements int64
	require.NoError(t, env.DB.Model(&models.Product{}).Where("id = ?", product.ID).Count(&products).Error)
	require.NoError(t, env.DB.Model(&models.UserEntitlement{}).Where("product_id = ?", product.ID).Count(&entitlements).Error)
	assert.EqualValues(t, 1, products)
	assert.EqualValues(t, 1, entitlements)

	res = setFree(env, "ar-cards", admin, cardID, false)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))
	assert.Equal(t, false, publicCard(t, env, stranger.Token, cardID)["is_unlocked"], "locked again")
	assert.Equal(t, true, publicCard(t, env, owner.Token, cardID)["is_unlocked"], "the earlier buyer keeps access")
}

func TestFreeContent_EditingACard_NeverChangesTheFreeFlag(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	cardID := newArCard(t, env, true)

	// A client that doesn't know about is_free just renames the card.
	res := env.PUT("/admin/content/ar-cards/"+cardID, admin, map[string]interface{}{
		"title": "Renamed", "type": "alphabet", "file_url": "https://cdn/x.glb",
	})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))

	var card models.ArCards
	require.NoError(t, env.DB.First(&card, "id = ?", cardID).Error)
	assert.Equal(t, "Renamed", card.Title)
	assert.True(t, card.IsFree, "saving a card must not turn a free card back into a paid one")
}

func TestFreeContent_NewCardCanBeCreatedFree(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)

	res := env.POST("/admin/content/ar-cards", admin, map[string]interface{}{
		"title": "Free lion", "type": "animal", "file_url": "https://cdn/lion.glb",
		"short_code": fmt.Sprintf("FL%d", fixtures.NextSeq()), "is_free": true,
	})
	require.Equal(t, http.StatusCreated, res.Code, string(res.Body))
	id := res.Data()["id"].(string)

	assert.Equal(t, true, publicCard(t, env, "", id)["is_unlocked"])
	access, _ := adminAccess(t, env, admin, "ar-cards", id)
	assert.Equal(t, "FREE", access)
}

func TestFreeContent_AccessReflectsFlagAndProduct(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)

	flagged := newArCard(t, env, true)
	noProduct := newArCard(t, env, false)
	paid, _ := soldArCard(t, env)
	withdrawn, withdrawnProduct := soldArCard(t, env)
	// Set explicitly: GORM applies the column default (true) to a false on create.
	require.NoError(t, env.DB.Model(&models.Product{}).Where("id = ?", withdrawnProduct.ID).Update("is_active", false).Error)

	for id, want := range map[string]string{
		flagged: "FREE", noProduct: "FREE_NO_PRODUCT", paid: "PAID", withdrawn: "PAID_INACTIVE",
	} {
		got, _ := adminAccess(t, env, admin, "ar-cards", id)
		assert.Equal(t, want, got, id)
	}

	// The list carries the same value.
	list := env.GET("/admin/content/ar-cards?per_page=100", admin)
	require.Equal(t, http.StatusOK, list.Code)
	seen := map[string]string{}
	for _, it := range list.JSON()["data"].([]interface{}) {
		m := it.(map[string]interface{})
		seen[m["id"].(string)] = m["access"].(string)
	}
	assert.Equal(t, "PAID_INACTIVE", seen[withdrawn])
	assert.Equal(t, "FREE", seen[flagged])
}

func TestFreeContent_Dongeng_AccessAndToggle(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	account := env.Register(t)

	d := fixtures.NewDongeng(t, env.DB, fixtures.Paid())
	require.NoError(t, env.DB.Model(&models.Dongeng{}).Where("id = ?", d.ID).Update("is_free", false).Error)
	product := fixtures.NewProduct(t, env.DB, fixtures.WithPrice(9_000), func(p *models.Product) {
		p.FeatureID = fixtures.FeatureID(t, env.DB, "DONGENG")
	})
	require.NoError(t, env.DB.Create(&models.ProductDongeng{ProductID: product.ID, DongengID: d.ID}).Error)
	id := d.ID.String()

	access, price := adminAccess(t, env, admin, "fairy-tales", id)
	assert.Equal(t, "PAID", access)
	assert.EqualValues(t, 9000, price)
	assert.Equal(t, false, env.GET("/fairy-tales/"+id, account.Token).Data()["is_unlocked"])

	require.Equal(t, http.StatusOK, setFree(env, "fairy-tales", admin, id, true).Code)
	assert.Equal(t, true, env.GET("/fairy-tales/"+id, account.Token).Data()["is_unlocked"])
	access, _ = adminAccess(t, env, admin, "fairy-tales", id)
	assert.Equal(t, "FREE", access)

	require.Equal(t, http.StatusOK, setFree(env, "fairy-tales", admin, id, false).Code)
	assert.Equal(t, false, env.GET("/fairy-tales/"+id, account.Token).Data()["is_unlocked"])
}

func TestFreeContent_SetFree_RejectsBadRequests(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	cardID := newArCard(t, env, false)

	assert.Equal(t, http.StatusNotFound, setFree(env, "ar-cards", admin, uuid.NewString(), true).Code)
	assert.Equal(t, http.StatusNotFound, setFree(env, "fairy-tales", admin, uuid.NewString(), true).Code)

	missing := env.do(http.MethodPatch, "/admin/content/ar-cards/"+cardID+"/free", admin, map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, missing.Code, "is_free is required, absent must not mean false")

	unauth := env.do(http.MethodPatch, "/admin/content/ar-cards/"+cardID+"/free", "", map[string]interface{}{"is_free": true})
	assert.Equal(t, http.StatusUnauthorized, unauth.Code)
}

func TestFreeContent_ProductsList_TagsProductsWhoseContentIsFree(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	cardID, product := soldArCard(t, env)

	find := func() interface{} {
		list := env.GET("/admin/products", admin)
		require.Equal(t, http.StatusOK, list.Code)
		for _, it := range list.JSON()["data"].([]interface{}) {
			m := it.(map[string]interface{})
			if m["id"] == product.ID.String() {
				return m["content_is_free"]
			}
		}
		t.Fatal("product not listed")
		return nil
	}
	assert.Equal(t, false, find())
	require.Equal(t, http.StatusOK, setFree(env, "ar-cards", admin, cardID, true).Code)
	assert.Equal(t, true, find())
}

func TestFreeContent_NoNewSingleProductOrderForFreeContent(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	account := env.Register(t)
	cardID, product := soldArCard(t, env)
	body := map[string]string{"product_id": product.ID.String()}

	require.Equal(t, http.StatusOK, env.POST("/payment/play/create-product", account.Token, body).Code)

	require.Equal(t, http.StatusOK, setFree(env, "ar-cards", admin, cardID, true).Code)
	var before int64
	require.NoError(t, env.DB.Model(&models.Order{}).Where("user_id = ?", account.ID).Count(&before).Error)

	res := env.POST("/payment/play/create-product", account.Token, body)
	assert.Equal(t, http.StatusBadRequest, res.Code, string(res.Body))
	assert.Equal(t, "content is free", res.JSON()["error"])

	var after int64
	require.NoError(t, env.DB.Model(&models.Order{}).Where("user_id = ?", account.ID).Count(&after).Error)
	assert.Equal(t, before, after, "no order may be created for free content")

	require.Equal(t, http.StatusOK, setFree(env, "ar-cards", admin, cardID, false).Code)
	assert.Equal(t, http.StatusOK, env.POST("/payment/play/create-product", account.Token, body).Code, "premium again, buyable again")
}

func TestFreeContent_PackageContainingAFreeItemStillSells(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	account := env.Register(t)
	cardID, _ := soldArCard(t, env)
	require.Equal(t, http.StatusOK, setFree(env, "ar-cards", admin, cardID, true).Code)

	pkg := fixtures.NewPackage(t, env.DB, fixtures.WithPackagePlayProductID(fmt.Sprintf("pkg_%d", fixtures.NextSeq())))
	res := env.POST("/payment/play/create", account.Token, map[string]string{"package_id": pkg.ID})
	assert.Equal(t, http.StatusOK, res.Code, string(res.Body))
}
