package api_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/services"
	"arunika_backend/tests/fixtures"
)

// Keranjang Belanja: the cart, checkout into one order, and the grant of
// every item from one Google Play purchase — through the real router and a
// real database, with Google faked by fixtures.FakePlay.

func enableCart(t *testing.T, env *Env) {
	t.Helper()
	require.NoError(t, env.DB.Model(&models.FeatureFlag{}).Where("key = ?", "cart").
		Update("is_enabled", true).Error)
}

// cartCard inserts a paid AR card and returns its product id.
func cartCard(t *testing.T, env *Env, priceIdr int64) uuid.UUID {
	t.Helper()
	cardID := paidArCard(t, env, priceIdr)
	var link models.ProductArCard
	require.NoError(t, env.DB.Where("ar_card_id = ?", cardID).First(&link).Error)
	return link.ProductID
}

// storeProduct registers the cart total product for totalIdr.
func storeProduct(t *testing.T, env *Env, totalIdr int64) string {
	t.Helper()
	sku := services.StoreProductSKU(totalIdr)
	require.NoError(t, env.DB.Exec(`INSERT INTO store_products (store, play_product_id, price_idr)
		VALUES ('google', ?, ?) ON CONFLICT DO NOTHING`, sku, totalIdr).Error)
	return sku
}

func userID(t *testing.T, account *Account) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(account.ID)
	require.NoError(t, err)
	return id
}

func addToCart(env *Env, account *Account, productID uuid.UUID) *Response {
	return env.POST("/cart/items", account.Token, map[string]string{"product_id": productID.String()})
}

func cartItems(r *Response) []interface{} {
	return r.Data()["items"].([]interface{})
}

func owns(t *testing.T, env *Env, user uuid.UUID, productID uuid.UUID) bool {
	t.Helper()
	has, err := models.HasEntitlement(env.DB, user, productID)
	require.NoError(t, err)
	return has
}

func TestCart_AddTwice_KeepsOneItemAndTotals(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	a := cartCard(t, env, 1_000)
	b := cartCard(t, env, 5_000)

	require.Equal(t, http.StatusOK, addToCart(env, account, a).Code)
	require.Equal(t, http.StatusOK, addToCart(env, account, a).Code, "adding again is a no-op, not an error")
	res := addToCart(env, account, b)
	require.Equal(t, http.StatusOK, res.Code, "body: %s", res.Body)

	cart := env.GET("/cart", account.Token)
	require.Equal(t, http.StatusOK, cart.Code)
	assert.Len(t, cartItems(cart), 2)
	assert.EqualValues(t, 2, cart.Data()["item_count"])
	assert.EqualValues(t, 6_000, cart.Data()["total_idr"])
}

func TestCart_FlagOff_RefusesAdd(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)
	res := addToCart(env, account, cartCard(t, env, 1_000))
	assert.Equal(t, http.StatusForbidden, res.Code)
	assert.Equal(t, "CART_DISABLED", res.JSON()["code"])
}

func TestCart_OwnedItem_Refused(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	p := cartCard(t, env, 1_000)
	require.NoError(t, models.GrantEntitlement(env.DB, userID(t, account), p, nil))

	res := addToCart(env, account, p)
	assert.Equal(t, http.StatusConflict, res.Code)
	assert.Equal(t, "ALREADY_OWNED", res.JSON()["code"])
}

func TestCart_Limits(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)

	for i := 0; i < services.CartMaxItems; i++ {
		require.Equal(t, http.StatusOK, addToCart(env, account, cartCard(t, env, 1_000)).Code)
	}
	res := addToCart(env, account, cartCard(t, env, 1_000))
	assert.Equal(t, http.StatusConflict, res.Code)
	assert.Equal(t, "CART_FULL", res.JSON()["code"])

	other := env.Register(t)
	require.Equal(t, http.StatusOK, addToCart(env, other, cartCard(t, env, 300_000)).Code)
	res = addToCart(env, other, cartCard(t, env, 300_000))
	assert.Equal(t, http.StatusConflict, res.Code)
	assert.Equal(t, "CART_TOTAL_LIMIT", res.JSON()["code"])
}

func TestCart_Subscriber_CartEmptiedWithNotice(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	require.Equal(t, http.StatusOK, addToCart(env, account, cartCard(t, env, 1_000)).Code)

	var user models.Parent
	require.NoError(t, env.DB.Where("id = ?", account.ID).First(&user).Error)
	fixtures.NewSubscription(t, env.DB, &user)

	cart := env.GET("/cart", account.Token)
	require.Equal(t, http.StatusOK, cart.Code)
	assert.Empty(t, cartItems(cart))
	notices := cart.Data()["notices"].([]interface{})
	require.Len(t, notices, 1)
	assert.Equal(t, "SUBSCRIPTION_ACTIVE", notices[0].(map[string]interface{})["code"])
}

// checkout creates an order from the cart and returns its id and SKU.
func checkout(t *testing.T, env *Env, account *Account) (string, string) {
	t.Helper()
	res := env.POST("/orders", account.Token, map[string]string{"store": "google"})
	require.Equal(t, http.StatusOK, res.Code, "body: %s", res.Body)
	return res.Data()["order_id"].(string), res.Data()["play_product_id"].(string)
}

func TestCart_CheckoutAndVerify_GrantsEveryItemAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	user := userID(t, account)
	a, b := cartCard(t, env, 1_000), cartCard(t, env, 5_000)
	sku := storeProduct(t, env, 6_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, a).Code)
	require.Equal(t, http.StatusOK, addToCart(env, account, b).Code)

	orderID, gotSKU := checkout(t, env, account)
	assert.Equal(t, sku, gotSKU)

	// Added while paying: not part of the order, must stay in the cart.
	c := cartCard(t, env, 2_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, c).Code)

	purchaseToken := token(t)
	fakePlay.PurchasedForOrder(purchaseToken, sku, orderID, 0)
	res := env.POST("/orders/"+orderID+"/verify", account.Token, map[string]string{"purchase_token": purchaseToken})
	require.Equal(t, http.StatusOK, res.Code, "body: %s", res.Body)
	assert.Equal(t, services.PhaseGranted, res.Data()["phase"])

	assert.True(t, owns(t, env, user, a))
	assert.True(t, owns(t, env, user, b))
	assert.False(t, owns(t, env, user, c))
	assert.True(t, fakePlay.Consumed(purchaseToken), "the purchase must be consumed so the total can be bought again")

	cart := env.GET("/cart", account.Token)
	items := cartItems(cart)
	require.Len(t, items, 1)
	assert.Equal(t, c.String(), items[0].(map[string]interface{})["product_id"])

	status := env.GET("/orders/"+orderID, account.Token)
	require.Equal(t, http.StatusOK, status.Code)
	assert.Equal(t, services.PhaseGranted, status.Data()["phase"])
	assert.Len(t, status.Data()["items"], 2)
}

func TestCart_Checkout_SameIdempotencyKeyReturnsSameOrder(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	storeProduct(t, env, 1_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, cartCard(t, env, 1_000)).Code)

	first := env.doWithHeaders(http.MethodPost, "/orders", account.Token,
		map[string]string{"store": "google"}, map[string]string{"Idempotency-Key": "k-1"})
	second := env.doWithHeaders(http.MethodPost, "/orders", account.Token,
		map[string]string{"store": "google"}, map[string]string{"Idempotency-Key": "k-1"})
	require.Equal(t, http.StatusOK, first.Code, "body: %s", first.Body)
	require.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, first.Data()["order_id"], second.Data()["order_id"])
}

func TestCart_Checkout_OwnedItemReportsChangeThenSucceeds(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	a, b := cartCard(t, env, 1_000), cartCard(t, env, 5_000)
	storeProduct(t, env, 5_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, a).Code)
	require.Equal(t, http.StatusOK, addToCart(env, account, b).Code)

	// Bought on another device.
	require.NoError(t, models.GrantEntitlement(env.DB, userID(t, account), a, nil))

	res := env.POST("/orders", account.Token, map[string]string{"store": "google"})
	require.Equal(t, http.StatusConflict, res.Code)
	assert.Equal(t, "CART_CHANGED", res.JSON()["code"])
	assert.Len(t, cartItems(res), 1)

	orderID, _ := checkout(t, env, account)
	var order models.Order
	require.NoError(t, env.DB.Where("id = ?", orderID).First(&order).Error)
	assert.EqualValues(t, 5_000, order.AmountIdr)
}

func TestCart_Checkout_PriceRiseShownOnceThenAccepted(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	p := cartCard(t, env, 1_000)
	storeProduct(t, env, 2_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, p).Code)
	require.NoError(t, env.DB.Model(&models.Product{}).Where("id = ?", p).Update("price_idr", 2_000).Error)

	res := env.POST("/orders", account.Token, map[string]string{"store": "google"})
	require.Equal(t, http.StatusConflict, res.Code)
	item := cartItems(res)[0].(map[string]interface{})
	assert.EqualValues(t, 1_000, item["price_changed_from"])
	assert.EqualValues(t, 2_000, item["price_idr"])

	_, sku := checkout(t, env, account)
	assert.Equal(t, services.StoreProductSKU(2_000), sku)
}

func TestCart_Checkout_PriceDropAppliesSilently(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	p := cartCard(t, env, 5_000)
	storeProduct(t, env, 3_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, p).Code)
	require.NoError(t, env.DB.Model(&models.Product{}).Where("id = ?", p).Update("price_idr", 3_000).Error)

	_, sku := checkout(t, env, account)
	assert.Equal(t, services.StoreProductSKU(3_000), sku)
}

func TestCart_Checkout_NoStoreProductForTotal(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	require.Equal(t, http.StatusOK, addToCart(env, account, cartCard(t, env, 7_000)).Code)

	res := env.POST("/orders", account.Token, map[string]string{"store": "google"})
	assert.Equal(t, http.StatusConflict, res.Code)
	assert.Equal(t, "AMOUNT_UNAVAILABLE", res.JSON()["code"])
}

func TestCart_SingleBeli_OrderOfOneSkipsCart(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	inCart, single := cartCard(t, env, 1_000), cartCard(t, env, 2_000)
	sku := storeProduct(t, env, 2_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, inCart).Code)

	res := env.POST("/orders", account.Token, map[string]interface{}{
		"store": "google", "items": []map[string]string{{"product_id": single.String()}},
	})
	require.Equal(t, http.StatusOK, res.Code, "body: %s", res.Body)
	assert.Equal(t, sku, res.Data()["play_product_id"])

	orderID := res.Data()["order_id"].(string)
	purchaseToken := token(t)
	fakePlay.PurchasedForOrder(purchaseToken, sku, orderID, 0)
	require.Equal(t, http.StatusOK, env.POST("/orders/"+orderID+"/verify", account.Token,
		map[string]string{"purchase_token": purchaseToken}).Code)

	assert.True(t, owns(t, env, userID(t, account), single))
	assert.Len(t, cartItems(env.GET("/cart", account.Token)), 1, "the cart is untouched by a single purchase")
}

func TestCart_CheaperPurchaseCannotUnlockBiggerOrder(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	p := cartCard(t, env, 100_000)
	storeProduct(t, env, 100_000)
	cheapSKU := storeProduct(t, env, 1_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, p).Code)
	orderID, _ := checkout(t, env, account)

	// A real Rp 1.000 purchase carrying this order's id.
	purchaseToken := token(t)
	fakePlay.PurchasedForOrder(purchaseToken, cheapSKU, orderID, 0)
	res := env.POST("/orders/"+orderID+"/verify", account.Token, map[string]string{"purchase_token": purchaseToken})
	assert.Equal(t, http.StatusBadRequest, res.Code)
	assert.False(t, owns(t, env, userID(t, account), p))
}

func TestCart_PurchaseOfAnotherOrderRejected(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	p := cartCard(t, env, 1_000)
	sku := storeProduct(t, env, 1_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, p).Code)
	orderID, _ := checkout(t, env, account)

	purchaseToken := token(t)
	fakePlay.PurchasedForOrder(purchaseToken, sku, uuid.NewString(), 0)
	res := env.POST("/orders/"+orderID+"/verify", account.Token, map[string]string{"purchase_token": purchaseToken})
	assert.Equal(t, http.StatusBadRequest, res.Code)
	assert.False(t, owns(t, env, userID(t, account), p))
}

func TestCart_PendingPayment_ProcessingThenSettledByWorker(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	p := cartCard(t, env, 1_000)
	sku := storeProduct(t, env, 1_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, p).Code)
	orderID, _ := checkout(t, env, account)

	purchaseToken := token(t)
	fakePlay.PurchasedForOrder(purchaseToken, sku, orderID, 2)
	res := env.POST("/orders/"+orderID+"/verify", account.Token, map[string]string{"purchase_token": purchaseToken})
	require.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, services.PhaseProcessing, res.Data()["phase"])
	assert.False(t, owns(t, env, userID(t, account), p))
	assert.Len(t, cartItems(env.GET("/cart", account.Token)), 1, "cart stays until the grant")

	// The payment completes after the price lock; the worker settles it at
	// the locked price.
	fakePlay.PurchasedForOrder(purchaseToken, sku, orderID, 0)
	require.NoError(t, env.DB.Model(&models.Order{}).Where("id = ?", orderID).
		Updates(map[string]interface{}{"created_at": time.Now().Add(-2 * time.Hour), "price_locked_until": time.Now().Add(-time.Hour)}).Error)
	svc := services.NewPaymentService(env.DB, services.NewEntitlementService(env.DB))
	svc.RunCartSettlement(context.Background())

	assert.True(t, owns(t, env, userID(t, account), p))
	assert.Equal(t, services.PhaseGranted, env.GET("/orders/"+orderID, account.Token).Data()["phase"])
}

func TestCart_ConcurrentSettlementGrantsOnce(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	a, b := cartCard(t, env, 1_000), cartCard(t, env, 2_000)
	sku := storeProduct(t, env, 3_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, a).Code)
	require.Equal(t, http.StatusOK, addToCart(env, account, b).Code)
	orderID, _ := checkout(t, env, account)
	purchaseToken := token(t)
	fakePlay.PurchasedForOrder(purchaseToken, sku, orderID, 0)
	require.NoError(t, env.DB.Model(&models.Order{}).Where("id = ?", orderID).
		Updates(map[string]interface{}{"purchase_token": purchaseToken, "created_at": time.Now().Add(-2 * time.Minute)}).Error)

	svc := services.NewPaymentService(env.DB, services.NewEntitlementService(env.DB))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			env.POST("/orders/"+orderID+"/verify", account.Token, map[string]string{"purchase_token": purchaseToken})
		}()
		go func() {
			defer wg.Done()
			svc.RunCartSettlement(context.Background())
		}()
	}
	wg.Wait()

	var grants int64
	require.NoError(t, env.DB.Model(&models.UserEntitlement{}).Where("source_order_id = ?", orderID).Count(&grants).Error)
	assert.EqualValues(t, 2, grants)
	var payments int64
	require.NoError(t, env.DB.Model(&models.Payment{}).Where("order_id = ? AND transaction_id = ?", orderID, purchaseToken).Count(&payments).Error)
	assert.EqualValues(t, 1, payments, "settled exactly once")
}

func TestCart_RefundRevokesEveryItemAndRebuyRestores(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	enableCart(t, env)
	account := env.Register(t)
	user := userID(t, account)
	a, b := cartCard(t, env, 1_000), cartCard(t, env, 2_000)
	sku := storeProduct(t, env, 3_000)
	require.Equal(t, http.StatusOK, addToCart(env, account, a).Code)
	require.Equal(t, http.StatusOK, addToCart(env, account, b).Code)
	orderID, _ := checkout(t, env, account)
	purchaseToken := token(t)
	fakePlay.PurchasedForOrder(purchaseToken, sku, orderID, 0)
	require.Equal(t, http.StatusOK, env.POST("/orders/"+orderID+"/verify", account.Token,
		map[string]string{"purchase_token": purchaseToken}).Code)

	fakePlay.Void(purchaseToken)
	svc := services.NewPaymentService(env.DB, services.NewEntitlementService(env.DB))
	_, err := svc.ReconcileVoidedPurchases(context.Background(), time.Now().Add(-24*time.Hour))
	require.NoError(t, err)

	assert.False(t, owns(t, env, user, a))
	assert.False(t, owns(t, env, user, b))

	// Buying a refunded item again restores it.
	single := storeProduct(t, env, 1_000)
	res := env.POST("/orders", account.Token, map[string]interface{}{
		"store": "google", "items": []map[string]string{{"product_id": a.String()}},
	})
	require.Equal(t, http.StatusOK, res.Code, "body: %s", res.Body)
	again := res.Data()["order_id"].(string)
	tok2 := token(t)
	fakePlay.PurchasedForOrder(tok2, single, again, 0)
	require.Equal(t, http.StatusOK, env.POST("/orders/"+again+"/verify", account.Token,
		map[string]string{"purchase_token": tok2}).Code)
	assert.True(t, owns(t, env, user, a))
}

func TestCart_Entitlements_ListsOwnedProducts(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)
	p := cartCard(t, env, 1_000)
	require.NoError(t, models.GrantEntitlement(env.DB, userID(t, account), p, nil))

	res := env.GET("/entitlements", account.Token)
	require.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, []interface{}{p.String()}, res.Data()["product_ids"])
}
