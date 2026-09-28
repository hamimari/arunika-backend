//go:build e2e

package e2e

import (
	"math"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Flow 1 — signup → login → child → home → browse.
func TestFlow1_SignupLoginAndBrowse(t *testing.T) {
	s := Up(t)

	signedUp, password := s.Signup()
	sess := s.Login(signedUp.Email, password)

	me := s.GET("/user/"+sess.UserID, sess.Token)
	require.Equal(t, http.StatusOK, me.Code, "body: %s", string(me.Body))
	assert.Contains(t, string(me.Body), "Budi", "the child created at signup must come back on the profile")

	cards := s.GET("/ar/cards", sess.Token)
	require.Equal(t, http.StatusOK, cards.Code)
	assert.NotNil(t, FindByID(cards.List(), FreeArCard1ID), "seeded free card must be browsable")
}

// Flow 2 — login → browse dongeng → open a free dongeng → play recorded.
func TestFlow2_OpenFreeDongengAndRecordPlay(t *testing.T) {
	s := Up(t)
	sess := s.Login(FreeUserEmail, SeedUserPassword)

	list := s.GET("/fairy-tales", sess.Token)
	require.Equal(t, http.StatusOK, list.Code, "body: %s", string(list.Body))
	assert.NotNil(t, FindByID(list.List(), FreeDongeng1ID))

	detail := s.GET("/fairy-tales/"+FreeDongeng1ID, sess.Token)
	require.Equal(t, http.StatusOK, detail.Code, "body: %s", string(detail.Body))

	play := s.POST("/fairy-tales/"+FreeDongeng1ID+"/play", sess.Token, map[string]interface{}{})
	require.Less(t, play.Code, 300, "record play: %s", string(play.Body))

	history := s.GET("/fairy-tales/history", sess.Token)
	require.Equal(t, http.StatusOK, history.Code, "body: %s", string(history.Body))
	assert.Contains(t, string(history.Body), FreeDongeng1ID, "a recorded play must appear in history")
}

// Flow 3 — paid AR card → Play purchase → verify → entitlement → unlocked.
// Replaces manual task 16.1 of add-monetization-entitlements.
func TestFlow3_PaidArCardPurchaseUnlocksIt(t *testing.T) {
	s := Up(t)
	sess := s.Login(FreeUserEmail, SeedUserPassword)

	before := FindByID(s.GET("/ar/cards", sess.Token).List(), PaidArCard1ID)
	require.NotNil(t, before)
	require.Equal(t, false, before["is_unlocked"], "a paid card must start locked for a free user")

	s.FakePlay.Purchased("e2e-tok-flow3")
	res := s.PlayPurchase(sess, "/payment/play/create-product", "product_id", PaidProduct1ID, PaidProduct1ID, "e2e-tok-flow3")
	require.Equal(t, http.StatusOK, res.Code, "verify: %s", string(res.Body))

	after := FindByID(s.GET("/ar/cards", sess.Token).List(), PaidArCard1ID)
	assert.Equal(t, true, after["is_unlocked"], "the purchased card must unlock")
	other := FindByID(s.GET("/ar/cards", sess.Token).List(), PaidArCard2ID)
	assert.Equal(t, false, other["is_unlocked"], "buying one card must not unlock another")
}

// Flow 4 — a bundle purchase grants every package item.
// Replaces manual tasks 16.2 and 16.5.
func TestFlow4_BundlePurchaseGrantsEveryItem(t *testing.T) {
	s := Up(t)
	sess := s.Login(FreeUserEmail, SeedUserPassword)

	s.FakePlay.Purchased("e2e-tok-flow4")
	res := s.PlayPurchase(sess, "/payment/play/create", "package_id", ContentBundlePackageID, BundlePlaySKU, "e2e-tok-flow4")
	require.Equal(t, http.StatusOK, res.Code, "verify: %s", string(res.Body))

	cards := s.GET("/ar/cards", sess.Token).List()
	for _, id := range []string{PaidArCard1ID, PaidArCard2ID} {
		card := FindByID(cards, id)
		require.NotNil(t, card)
		assert.Equal(t, true, card["is_unlocked"], "bundle item %s must be unlocked", id)
	}
	assert.Equal(t, 2, s.Count(`SELECT COUNT(*) FROM user_entitlements WHERE user_id = $1`, FreeUserID))
}

// Flow 5 — admin creates and publishes content → the app's API returns it.
func TestFlow5_AdminPublishedContentReachesTheApp(t *testing.T) {
	s := Up(t)
	admin := s.AdminLogin()
	user := s.Login(FreeUserEmail, SeedUserPassword)

	created := s.POST("/admin/content/fairy-tales", admin, map[string]interface{}{
		"title": "E2E Admin Dongeng", "image_url": "https://e2e.test/i.png",
		"audio_url": "https://e2e.test/a.mp3", "is_free": true, "duration": 120,
	})
	require.Equal(t, http.StatusCreated, created.Code, "body: %s", string(created.Body))
	id := created.Data()["id"].(string)

	hide := s.PATCH("/admin/content/fairy-tales/"+id+"/visibility", admin, map[string]bool{"hidden": true})
	require.Equal(t, http.StatusOK, hide.Code)
	assert.Nil(t, FindByID(s.GET("/fairy-tales", user.Token).List(), id), "a hidden dongeng must not reach the app")

	show := s.PATCH("/admin/content/fairy-tales/"+id+"/visibility", admin, map[string]bool{"hidden": false})
	require.Equal(t, http.StatusOK, show.Code)
	assert.NotNil(t, FindByID(s.GET("/fairy-tales", user.Token).List(), id), "a published dongeng must reach the app")
}

// Webhook replay — the same notification and the same verification twice must
// not duplicate payments or entitlements. Replaces manual task 16.3.
func TestWebhookReplay_IsIdempotent(t *testing.T) {
	s := Up(t)
	sess := s.Login(FreeUserEmail, SeedUserPassword)

	expiry := strconv.FormatInt(time.Now().Add(30*24*time.Hour).UnixMilli(), 10)
	s.FakePlay.SubscriptionActive("e2e-tok-sub", expiry)

	res := s.PlayPurchase(sess, "/payment/play/create", "package_id", SubscriptionPackageID, SubscriptionPlaySKU, "e2e-tok-sub")
	require.Equal(t, http.StatusOK, res.Code, "verify: %s", string(res.Body))

	const renewed = 2 // SUBSCRIPTION_RENEWED
	for i := 0; i < 2; i++ {
		require.Equal(t, http.StatusOK, s.RTDN(renewed, SubscriptionPlaySKU, "e2e-tok-sub").Code)
	}
	again := s.POST("/payment/play/verify", sess.Token, map[string]string{
		"product_id": SubscriptionPlaySKU, "purchase_token": "e2e-tok-sub",
	})
	t.Logf("re-verify without order id: %d %s", again.Code, string(again.Body))

	assert.Equal(t, 1, s.Count(`SELECT COUNT(*) FROM payments WHERE transaction_id = $1`, "e2e-tok-sub"))
	assert.Equal(t, 1, s.Count(`SELECT COUNT(*) FROM user_subscriptions WHERE user_id = $1`, FreeUserID))
}

// GET /premium/packs with and without ?type. Replaces manual task 16.4.
func TestPremiumPacks_TypeFilter(t *testing.T) {
	s := Up(t)

	all := s.GET("/premium/packs", "").List()
	assert.NotNil(t, FindByID(all, ContentBundlePackageID))
	assert.NotNil(t, FindByID(all, SubscriptionPackageID))

	content := s.GET("/premium/packs?type=content", "").List()
	assert.NotNil(t, FindByID(content, ContentBundlePackageID))
	assert.Nil(t, FindByID(content, SubscriptionPackageID), "type=content must exclude subscriptions")

	subs := s.GET("/premium/packs?type=subscription", "").List()
	assert.NotNil(t, FindByID(subs, SubscriptionPackageID))
	assert.Nil(t, FindByID(subs, ContentBundlePackageID), "type=subscription must exclude bundles")
}

// Admin manual grant. Replaces manual task 16.6.
func TestAdminManualGrantAndRevoke(t *testing.T) {
	s := Up(t)
	admin := s.AdminLogin()
	user := s.Login(FreeUserEmail, SeedUserPassword)

	locked := FindByID(s.GET("/ar/cards", user.Token).List(), PaidArCard1ID)
	require.Equal(t, false, locked["is_unlocked"])

	grant := s.PATCH("/admin/users/"+FreeUserID+"/permission", admin,
		map[string]interface{}{"action": "grant", "duration_days": 30})
	require.Equal(t, http.StatusOK, grant.Code, "body: %s", string(grant.Body))
	unlocked := FindByID(s.GET("/ar/cards", user.Token).List(), PaidArCard1ID)
	assert.Equal(t, true, unlocked["is_unlocked"], "a manual grant must unlock paid content")

	revoke := s.PATCH("/admin/users/"+FreeUserID+"/permission", admin, map[string]string{"action": "revoke"})
	require.Equal(t, http.StatusOK, revoke.Code)
	relocked := FindByID(s.GET("/ar/cards", user.Token).List(), PaidArCard1ID)
	assert.Equal(t, false, relocked["is_unlocked"], "a revoke must lock it again")
}

// A subscription purchase unlocks all paid content through the blanket
// subscription, without writing a per-item entitlement row.
// Replaces manual task 16.2.
func TestSubscriptionPurchase_UnlocksEverythingWithoutEntitlementRows(t *testing.T) {
	s := Up(t)
	sess := s.Login(FreeUserEmail, SeedUserPassword)

	expiry := strconv.FormatInt(time.Now().Add(30*24*time.Hour).UnixMilli(), 10)
	s.FakePlay.SubscriptionActive("e2e-tok-sub-unlock", expiry)
	res := s.PlayPurchase(sess, "/payment/play/create", "package_id", SubscriptionPackageID, SubscriptionPlaySKU, "e2e-tok-sub-unlock")
	require.Equal(t, http.StatusOK, res.Code, "verify: %s", string(res.Body))

	cards := s.GET("/ar/cards", sess.Token).List()
	for _, id := range []string{PaidArCard1ID, PaidArCard2ID} {
		card := FindByID(cards, id)
		require.NotNil(t, card)
		assert.Equal(t, true, card["is_unlocked"], "subscription must unlock paid card %s", id)
	}
	assert.Equal(t, 0, s.Count(`SELECT COUNT(*) FROM user_entitlements WHERE user_id = $1`, FreeUserID),
		"a subscription unlocks via user_subscriptions, not per-item entitlement rows")
	assert.Equal(t, 1, s.Count(`SELECT COUNT(*) FROM user_subscriptions WHERE user_id = $1 AND status = 'premium'`, FreeUserID))
}

// A promo set from the backoffice reaches the app's package list as a
// display-only strike price — and the order still charges the real price.
func TestStrikePrice_AdminPromoReachesTheAppButIsNeverCharged(t *testing.T) {
	s := Up(t)
	admin := s.AdminLogin()

	rule := s.PUT("/admin/strike-price-rules/PACKAGE", admin, map[string]interface{}{
		"mode":    "PERCENT",
		"value":   20,
		"ends_at": time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339),
	})
	require.Equal(t, http.StatusOK, rule.Code, "body: %s", string(rule.Body))
	assert.Equal(t, "ACTIVE", rule.Data()["status"])

	bundle := FindByID(s.GET("/premium/packs", "").List(), ContentBundlePackageID)
	require.NotNil(t, bundle)
	price := bundle["price_idr"].(float64)
	require.NotNil(t, bundle["strike_price_idr"], "the promo must reach the public package list")
	strike := bundle["strike_price_idr"].(float64)
	assert.Greater(t, strike, price)
	// The badge is derived from the rounded strike price, so it can differ
	// from the configured 20% by a point.
	assert.EqualValues(t, math.Round((strike-price)/strike*100), bundle["discount_percent"])
	assert.Nil(t, bundle["strike_mode"], "how the promo is configured stays private")

	sess := s.Login(FreeUserEmail, SeedUserPassword)
	order := s.POST("/payment/play/create", sess.Token, map[string]string{"package_id": ContentBundlePackageID})
	require.Equal(t, http.StatusOK, order.Code, "body: %s", string(order.Body))
	assert.Equal(t, int(price), s.Count(`SELECT amount_idr FROM orders WHERE id = $1`, order.Data()["order_id"]),
		"the order charges price_idr, never the strike price")
}

// An active subscriber already has everything: a further purchase is
// refused before any order exists.
func TestSubscriber_IsRefusedAPurchaseTheyDoNotNeed(t *testing.T) {
	s := Up(t)
	sess := s.Login(FreeUserEmail, SeedUserPassword)

	expiry := strconv.FormatInt(time.Now().Add(30*24*time.Hour).UnixMilli(), 10)
	s.FakePlay.SubscriptionActive("e2e-tok-sub-guard", expiry)
	res := s.PlayPurchase(sess, "/payment/play/create", "package_id", SubscriptionPackageID, SubscriptionPlaySKU, "e2e-tok-sub-guard")
	require.Equal(t, http.StatusOK, res.Code, "verify: %s", string(res.Body))

	refused := s.POST("/payment/play/create-product", sess.Token, map[string]string{"product_id": PaidProduct1ID})
	assert.Equal(t, http.StatusConflict, refused.Code, "body: %s", string(refused.Body))
	assert.Equal(t, "SUBSCRIPTION_ACTIVE", refused.JSON()["code"])

	profile := s.GET("/user/"+FreeUserID, sess.Token).Data()
	sub := profile["subscription"].(map[string]interface{})
	assert.Equal(t, "google_play", sub["provider"])
	assert.Equal(t, false, sub["can_renew"], "a month out, and Play renews it anyway")
}
