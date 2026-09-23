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

// The purchase chain: order → Google Play verification → settled order →
// entitlement → unlocked content. This is the most expensive thing in the
// product to get wrong in either direction — granting too much loses revenue,
// granting too little breaks someone who paid — and until now it was covered
// only by sqlmock tests that asserted against their own expectation lists.
//
// Nothing here needs a Google credential: fixtures.FakePlay serves both the
// Android Publisher API and the OAuth token endpoint locally, reached through
// the ANDROID_PUBLISHER_BASE_URL seam the verifier already supports.

// token returns a purchase token unique to this test, so parallel tests can
// share the one fake Play server without colliding.
func token(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("tok-%d", fixtures.NextSeq())
}

// playProduct creates a sellable product mapped to a Play SKU.
func playProduct(t *testing.T, env *Env) (*models.Product, string) {
	t.Helper()
	sku := fmt.Sprintf("sku_%d", fixtures.NextSeq())
	return fixtures.NewProduct(t, env.DB, fixtures.WithPlayProductID(sku)), sku
}

func TestPurchase_ValidToken_SettlesOrderAndGrantsEntitlement(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	account := env.Register(t)
	product, _ := playProduct(t, env)

	created := env.POST("/payment/play/create-product", account.Token,
		map[string]string{"product_id": product.ID.String()})
	require.Equal(t, http.StatusOK, created.Code, "body: %s", string(created.Body))
	orderID := created.Data()["order_id"].(string)

	purchaseToken := token(t)
	fakePlay.Purchased(purchaseToken)

	verified := env.POST("/payment/play/verify", account.Token, map[string]string{
		"order_id":       orderID,
		"product_id":     product.ID.String(),
		"purchase_token": purchaseToken,
	})
	require.Equal(t, http.StatusOK, verified.Code, "body: %s", string(verified.Body))

	// The order settled...
	var order models.Order
	require.NoError(t, env.DB.Where("id = ?", orderID).First(&order).Error)
	assert.Equal(t, "PAID", order.Status)

	// ...and access actually followed.
	var user models.Parent
	require.NoError(t, env.DB.Where("email_address = ?", account.Email).First(&user).Error)
	has, err := models.HasEntitlement(env.DB, user.ID, product.ID)
	require.NoError(t, err)
	assert.True(t, has, "a settled purchase must grant access to what was bought")
}

// Webhook replays, retried verifications and double taps all land here. The
// guarantee is the real UNIQUE (user_id, product_id) index.
func TestPurchase_SameTokenVerifiedTwice_GrantsOnlyOneEntitlement(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	account := env.Register(t)
	product, _ := playProduct(t, env)

	created := env.POST("/payment/play/create-product", account.Token,
		map[string]string{"product_id": product.ID.String()})
	require.Equal(t, http.StatusOK, created.Code)
	orderID := created.Data()["order_id"].(string)

	purchaseToken := token(t)
	fakePlay.Purchased(purchaseToken)

	body := map[string]string{
		"order_id":       orderID,
		"product_id":     product.ID.String(),
		"purchase_token": purchaseToken,
	}
	require.Equal(t, http.StatusOK, env.POST("/payment/play/verify", account.Token, body).Code)
	second := env.POST("/payment/play/verify", account.Token, body)
	require.Equal(t, http.StatusOK, second.Code, "a replay must be idempotent, not an error")

	var user models.Parent
	require.NoError(t, env.DB.Where("email_address = ?", account.Email).First(&user).Error)

	var count int64
	require.NoError(t, env.DB.Model(&models.UserEntitlement{}).
		Where("user_id = ? AND product_id = ?", user.ID, product.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "replaying a purchase must not grant it twice")
}

func TestPurchase_UnknownToken_IsRejectedAndGrantsNothing(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	account := env.Register(t)
	product, _ := playProduct(t, env)

	created := env.POST("/payment/play/create-product", account.Token,
		map[string]string{"product_id": product.ID.String()})
	require.Equal(t, http.StatusOK, created.Code)
	orderID := created.Data()["order_id"].(string)

	// Never registered with the fake, so Play reports it as unknown.
	res := env.POST("/payment/play/verify", account.Token, map[string]string{
		"order_id":       orderID,
		"product_id":     product.ID.String(),
		"purchase_token": token(t),
	})

	assert.NotEqual(t, http.StatusOK, res.Code, "an unverifiable purchase must not succeed")

	var order models.Order
	require.NoError(t, env.DB.Where("id = ?", orderID).First(&order).Error)
	assert.NotEqual(t, "PAID", order.Status, "an unverified order must not settle")

	var user models.Parent
	require.NoError(t, env.DB.Where("email_address = ?", account.Email).First(&user).Error)
	has, err := models.HasEntitlement(env.DB, user.ID, product.ID)
	require.NoError(t, err)
	assert.False(t, has, "a rejected purchase must grant nothing")
}

func TestPurchase_CanceledPurchase_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	account := env.Register(t)
	product, _ := playProduct(t, env)

	created := env.POST("/payment/play/create-product", account.Token,
		map[string]string{"product_id": product.ID.String()})
	require.Equal(t, http.StatusOK, created.Code)

	purchaseToken := token(t)
	fakePlay.Canceled(purchaseToken)

	res := env.POST("/payment/play/verify", account.Token, map[string]string{
		"order_id":       created.Data()["order_id"].(string),
		"product_id":     product.ID.String(),
		"purchase_token": purchaseToken,
	})

	assert.NotEqual(t, http.StatusOK, res.Code, "a canceled Play purchase must not grant access")
}

// Buying a bundle must unlock every product in it — the fan-out over
// premium_package_items is where a silent under-grant would hide.
func TestPurchase_ContentBundle_GrantsEveryItem(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	account := env.Register(t)
	sku := fmt.Sprintf("pkg_%d", fixtures.NextSeq())
	pkg, products := fixtures.NewPackageWithItems(t, env.DB, 3,
		fixtures.WithPackagePlayProductID(sku))

	created := env.POST("/payment/play/create", account.Token,
		map[string]string{"package_id": pkg.ID})
	require.Equal(t, http.StatusOK, created.Code, "body: %s", string(created.Body))
	orderID := created.Data()["order_id"].(string)

	purchaseToken := token(t)
	fakePlay.Purchased(purchaseToken)

	verified := env.POST("/payment/play/verify", account.Token, map[string]string{
		"order_id":       orderID,
		"product_id":     sku,
		"purchase_token": purchaseToken,
	})
	require.Equal(t, http.StatusOK, verified.Code, "body: %s", string(verified.Body))

	var user models.Parent
	require.NoError(t, env.DB.Where("email_address = ?", account.Email).First(&user).Error)

	for _, product := range products {
		has, err := models.HasEntitlement(env.DB, user.ID, product.ID)
		require.NoError(t, err)
		assert.True(t, has, "bundle item %s must be unlocked by the package purchase", product.ID)
	}
}

// A purchase belongs to the account that made it. If a token could be
// replayed by someone else, one purchase would unlock content for many
// accounts — the schema does not prevent this (there is no unique index on
// orders.purchase_token), so the application must.
func TestPurchase_TokenReusedByAnotherAccount_GrantsNothing(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	buyer := env.Register(t)
	attacker := env.Register(t)
	product, _ := playProduct(t, env)

	buyerOrder := env.POST("/payment/play/create-product", buyer.Token,
		map[string]string{"product_id": product.ID.String()})
	require.Equal(t, http.StatusOK, buyerOrder.Code)

	purchaseToken := token(t)
	fakePlay.Purchased(purchaseToken)

	require.Equal(t, http.StatusOK, env.POST("/payment/play/verify", buyer.Token, map[string]string{
		"order_id":       buyerOrder.Data()["order_id"].(string),
		"product_id":     product.ID.String(),
		"purchase_token": purchaseToken,
	}).Code)

	// The attacker starts their own order and replays the buyer's token.
	attackerOrder := env.POST("/payment/play/create-product", attacker.Token,
		map[string]string{"product_id": product.ID.String()})
	require.Equal(t, http.StatusOK, attackerOrder.Code)

	env.POST("/payment/play/verify", attacker.Token, map[string]string{
		"order_id":       attackerOrder.Data()["order_id"].(string),
		"product_id":     product.ID.String(),
		"purchase_token": purchaseToken,
	})

	var attackerUser models.Parent
	require.NoError(t, env.DB.Where("email_address = ?", attacker.Email).First(&attackerUser).Error)
	has, err := models.HasEntitlement(env.DB, attackerUser.ID, product.ID)
	require.NoError(t, err)
	assert.False(t, has,
		"one purchase token must never unlock content for a second account")
}

// Entitlements are keyed to the user, not the device or session — so a
// reinstall followed by a fresh sign-in must restore access.
func TestPurchase_EntitlementSurvivesReinstallAndRelogin(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	account := env.Register(t)
	product, _ := playProduct(t, env)

	created := env.POST("/payment/play/create-product", account.Token,
		map[string]string{"product_id": product.ID.String()})
	require.Equal(t, http.StatusOK, created.Code)

	purchaseToken := token(t)
	fakePlay.Purchased(purchaseToken)
	require.Equal(t, http.StatusOK, env.POST("/payment/play/verify", account.Token, map[string]string{
		"order_id":       created.Data()["order_id"].(string),
		"product_id":     product.ID.String(),
		"purchase_token": purchaseToken,
	}).Code)

	// Reinstall: the old session is gone, the user signs in again.
	login := env.Login(t, account.Email, account.Password)
	require.Equal(t, http.StatusOK, login.Code)

	var user models.Parent
	require.NoError(t, env.DB.Where("email_address = ?", account.Email).First(&user).Error)
	has, err := models.HasEntitlement(env.DB, user.ID, product.ID)
	require.NoError(t, err)
	assert.True(t, has, "a new session must not lose previously purchased content")
}

func TestPurchase_RequiresAuthentication(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	res := env.POST("/payment/play/verify", "", map[string]string{
		"product_id":     uuid.NewString(),
		"purchase_token": token(t),
	})

	assert.Equal(t, http.StatusUnauthorized, res.Code)
}

func TestPurchase_UnmappedProduct_IsRejectedBeforeAnyOrderExists(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	account := env.Register(t)
	// No play_product_id, so it cannot be bought through Play Billing.
	product := fixtures.NewProduct(t, env.DB)

	res := env.POST("/payment/play/create-product", account.Token,
		map[string]string{"product_id": product.ID.String()})

	assert.Equal(t, http.StatusBadRequest, res.Code,
		"a product with no Play SKU must not produce an order")
}
