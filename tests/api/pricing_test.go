package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/services"
	"arunika_backend/tests/fixtures"
)

// Promotional strike prices and the active-subscription purchase guard,
// through the real router and a real database.

// paidArCard inserts an AR card sold as a product at priceIdr.
func paidArCard(t *testing.T, env *Env, priceIdr int64) string {
	t.Helper()
	cardID := uuid.NewString()
	require.NoError(t, env.DB.Create(&models.ArCards{
		ID:        cardID,
		Type:      "animal",
		Title:     fmt.Sprintf("Kartu %d", fixtures.NextSeq()),
		FileURL:   "https://example.test/model.glb",
		ShortCode: fmt.Sprintf("API%d", fixtures.NextSeq()),
	}).Error)
	product := fixtures.NewProduct(t, env.DB, fixtures.WithPrice(priceIdr))
	require.NoError(t, env.DB.Create(&models.ProductArCard{ProductID: product.ID, ArCardID: cardID}).Error)
	return cardID
}

// paidDongeng inserts a dongeng sold as a product at priceIdr.
func paidDongeng(t *testing.T, env *Env, priceIdr int64) uuid.UUID {
	t.Helper()
	dongeng := fixtures.NewDongeng(t, env.DB, fixtures.Paid())
	product := fixtures.NewProduct(t, env.DB, fixtures.WithPrice(priceIdr), func(p *models.Product) {
		p.FeatureID = fixtures.FeatureID(t, env.DB, "DONGENG")
	})
	require.NoError(t, env.DB.Create(&models.ProductDongeng{ProductID: product.ID, DongengID: dongeng.ID}).Error)
	return dongeng.ID
}

func setRule(t *testing.T, env *Env, scope, mode string, value int) {
	t.Helper()
	end := time.Now().AddDate(0, 0, 7)
	_, err := services.NewStrikePriceService(env.DB).UpdateRule(scope,
		services.UpdateRuleInput{Mode: mode, Value: value, EndsAt: &end})
	require.NoError(t, err)
}

func findByID(t *testing.T, items []interface{}, id string) map[string]interface{} {
	t.Helper()
	for _, it := range items {
		m := it.(map[string]interface{})
		if m["id"] == id {
			return m
		}
	}
	t.Fatalf("item %s not in response", id)
	return nil
}

func TestPublicAPIs_ShowTheStrikePriceWhileAPromoRuns(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	cardID := paidArCard(t, env, 15000)
	dongengID := paidDongeng(t, env, 39000)
	setRule(t, env, models.StrikeScopeArCard, models.StrikeModePercent, 20)
	setRule(t, env, models.StrikeScopeDongeng, models.StrikeModeFixed, 10000)

	cards := env.GET("/ar/cards", "")
	require.Equal(t, http.StatusOK, cards.Code, string(cards.Body))
	card := findByID(t, cards.JSON()["data"].([]interface{}), cardID)
	assert.EqualValues(t, 15000, card["price_idr"])
	assert.EqualValues(t, 19000, card["strike_price_idr"])
	assert.EqualValues(t, 21, card["discount_percent"])
	assert.NotNil(t, card["promo_ends_at"])

	detail := env.GET("/ar/cards/"+cardID, "")
	require.Equal(t, http.StatusOK, detail.Code, string(detail.Body))
	assert.EqualValues(t, 19000, detail.JSON()["strike_price_idr"], "the detail endpoint returns the card unwrapped")

	tales := env.GET("/fairy-tales", "")
	require.Equal(t, http.StatusOK, tales.Code, string(tales.Body))
	tale := findByID(t, tales.JSON()["data"].([]interface{}), dongengID.String())
	assert.EqualValues(t, 39000, tale["price_idr"])
	assert.EqualValues(t, 49000, tale["strike_price_idr"])
}

func TestPublicAPIs_DropTheStrikePriceOnceThePromoEnds(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	cardID := paidArCard(t, env, 15000)
	setRule(t, env, models.StrikeScopeArCard, models.StrikeModePercent, 20)
	// The admin API refuses a past end date; move it back directly to model
	// the promo running out.
	require.NoError(t, env.DB.Exec(
		`UPDATE strike_price_rules SET ends_at = NOW() - INTERVAL '1 minute' WHERE scope = 'AR_CARD'`).Error)

	cards := env.GET("/ar/cards", "")
	require.Equal(t, http.StatusOK, cards.Code)
	card := findByID(t, cards.JSON()["data"].([]interface{}), cardID)
	assert.EqualValues(t, 15000, card["price_idr"])
	assert.Nil(t, card["strike_price_idr"])
	assert.Nil(t, card["promo_ends_at"])
}

func subscribe(t *testing.T, env *Env, account *Account, expiresIn time.Duration) {
	t.Helper()
	var parent models.Parent
	require.NoError(t, env.DB.First(&parent, "id = ?", account.ID).Error)
	fixtures.NewSubscription(t, env.DB, &parent, fixtures.ExpiringAt(time.Now().Add(expiresIn)))
}

func TestSubscriber_IsNotChargedForSomethingTheyAlreadyHave(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)
	subscribe(t, env, account, 20*24*time.Hour)
	product, _ := playProduct(t, env)

	res := env.POST("/payment/play/create-product", account.Token,
		map[string]string{"product_id": product.ID.String()})

	assert.Equal(t, http.StatusConflict, res.Code, string(res.Body))
	assert.Equal(t, "SUBSCRIPTION_ACTIVE", res.JSON()["code"])
	var orders int64
	require.NoError(t, env.DB.Model(&models.Order{}).Where("user_id = ?", account.ID).Count(&orders).Error)
	assert.Zero(t, orders, "no order may be created for an active subscriber")
}

func TestSubscriber_CanRenewOnlyInTheLastDays(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	monthly := fixtures.NewPackage(t, env.DB, fixtures.AsSubscription(30), fixtures.WithPackagePlayProductID(
		fmt.Sprintf("sub_%d", fixtures.NextSeq())))
	renew := map[string]interface{}{"plan_name": monthly.Name, "amount": monthly.PriceIdr}
	// Renewing through Midtrans exists only with alternative billing on.
	enableAlternativeBilling(t, env)

	early := env.Register(t)
	subscribe(t, env, early, 20*24*time.Hour)
	res := env.POST("/payment/create", early.Token, renew)
	assert.Equal(t, http.StatusConflict, res.Code, "20 days early: %s", string(res.Body))

	due := env.Register(t)
	subscribe(t, env, due, 5*24*time.Hour)
	res = env.POST("/payment/create", due.Token, renew)
	// Past the guard. (Midtrans itself isn't reachable from this harness, so
	// only "not refused as SUBSCRIPTION_ACTIVE" is asserted.)
	assert.NotEqual(t, http.StatusConflict, res.Code, "inside the window: %s", string(res.Body))

	// A new Google Play subscription order is never the renewal path.
	res = env.POST("/payment/play/create", due.Token, map[string]string{"package_id": monthly.ID})
	assert.Equal(t, http.StatusConflict, res.Code, string(res.Body))
}

func TestProfile_ReportsTheRenewalWindow(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	check := func(expiresIn time.Duration, wantCanRenew bool) {
		account := env.Register(t)
		subscribe(t, env, account, expiresIn)
		res := env.GET("/user/"+account.ID, account.Token)
		require.Equal(t, http.StatusOK, res.Code, string(res.Body))
		sub := res.Data()["subscription"].(map[string]interface{})
		assert.Equal(t, wantCanRenew, sub["can_renew"], "expires in %s", expiresIn)
		assert.Equal(t, "midtrans", sub["provider"])
		assert.Equal(t, false, sub["auto_renew"])
		assert.NotNil(t, sub["renewable_from"])
	}
	// A Midtrans subscription renews through the Midtrans checkout, so it is
	// only renewable while alternative billing is on.
	check(5*24*time.Hour, false)
	enableAlternativeBilling(t, env)
	check(20*24*time.Hour, false)
	check(5*24*time.Hour, true)
}

func enableAlternativeBilling(t *testing.T, env *Env) {
	t.Helper()
	require.NoError(t, env.DB.Exec(
		`UPDATE app_feature_flags SET is_enabled = TRUE WHERE key = 'alternative_billing'`).Error)
}

func TestMidtransCheckout_IsClosedUntilAlternativeBillingIsEnabled(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)
	product := fixtures.NewProduct(t, env.DB)
	pkg := fixtures.NewPackage(t, env.DB)

	for path, body := range map[string]interface{}{
		"/payment/create-product": map[string]string{"product_id": product.ID.String()},
		"/payment/create":         map[string]interface{}{"plan_name": pkg.Name, "amount": pkg.PriceIdr},
	} {
		res := env.POST(path, account.Token, body)
		assert.Equal(t, http.StatusForbidden, res.Code, "%s: %s", path, string(res.Body))
		assert.Equal(t, "ALTERNATIVE_BILLING_DISABLED", res.JSON()["code"], path)
	}
	var orders int64
	require.NoError(t, env.DB.Model(&models.Order{}).Where("user_id = ?", account.ID).Count(&orders).Error)
	assert.Zero(t, orders, "no Midtrans order may be created while alternative billing is off")

	// Google Play is unaffected.
	sku := fmt.Sprintf("sku_%d", fixtures.NextSeq())
	playable := fixtures.NewProduct(t, env.DB, fixtures.WithPlayProductID(sku))
	res := env.POST("/payment/play/create-product", account.Token, map[string]string{"product_id": playable.ID.String()})
	assert.Equal(t, http.StatusOK, res.Code, string(res.Body))

	enableAlternativeBilling(t, env)
	res = env.POST("/payment/create-product", account.Token, map[string]string{"product_id": product.ID.String()})
	assert.NotEqual(t, http.StatusForbidden, res.Code,
		"with the flag on the checkout is open again (Midtrans itself isn't reachable here): %s", string(res.Body))
}
