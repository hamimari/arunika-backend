package security_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// ── 7.4 Purchase tampering ──────────────────────────────────────────────────
//
// Every one of these is an attacker holding a real, valid Play purchase token
// (or none) and trying to turn it into access they did not pay for.

func purchaseToken() string { return fmt.Sprintf("sec-tok-%d", fixtures.NextSeq()) }

func playProduct(t *testing.T, env *Env) *models.Product {
	t.Helper()
	return fixtures.NewProduct(t, env.DB, fixtures.WithPlayProductID(fmt.Sprintf("sku_%d", fixtures.NextSeq())))
}

func (e *Env) createOrder(t *testing.T, a *Account, p *models.Product) string {
	t.Helper()
	res := e.POST("/payment/play/create-product", a.Token, map[string]string{"product_id": p.ID.String()})
	require.Equal(t, http.StatusOK, res.Code, "create order: %s", string(res.Body))
	return res.Data()["order_id"].(string)
}

func (e *Env) verify(a *Account, orderID, productID, token string) *Response {
	return e.POST("/payment/play/verify", a.Token, map[string]string{
		"order_id": orderID, "product_id": productID, "purchase_token": token,
	})
}

func (e *Env) entitled(t *testing.T, a *Account, p *models.Product) bool {
	t.Helper()
	has, err := models.HasEntitlement(e.DB, a.ID, p.ID)
	require.NoError(t, err)
	return has
}

func (e *Env) orderStatus(t *testing.T, orderID string) string {
	t.Helper()
	var o models.Order
	require.NoError(t, e.DB.First(&o, "id = ?", orderID).Error)
	return o.Status
}

// Buy the cheap product, then claim the expensive one in the request body.
func TestPurchaseTampering_ProductIDSwap_GrantsOnlyWhatTheOrderCovers(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	attacker := env.Register(t)
	cheap, expensive := playProduct(t, env), playProduct(t, env)

	orderID := env.createOrder(t, attacker, cheap)
	tok := purchaseToken()
	fakePlay.Purchased(tok)

	env.verify(attacker, orderID, expensive.ID.String(), tok)

	assert.False(t, env.entitled(t, attacker, expensive),
		"the product named in the verify body must never override the product the order was created for")
}

// A token Google says belongs to another app (or that it has never seen for
// this app) must not settle an order, whatever the client claims.
func TestPurchaseTampering_TokenForAnotherPackage_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	attacker := env.Register(t)
	product := playProduct(t, env)
	orderID := env.createOrder(t, attacker, product)

	tok := purchaseToken()
	// Google answers 404 for a token issued to a different application ID.
	fakePlay.Rejects(tok, http.StatusNotFound)

	res := env.verify(attacker, orderID, product.ID.String(), tok)

	assert.GreaterOrEqual(t, res.Code, 400)
	assert.Equal(t, "PENDING", env.orderStatus(t, orderID))
	assert.False(t, env.entitled(t, attacker, product))
}

func TestPurchaseTampering_FabricatedToken_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	attacker := env.Register(t)
	product := playProduct(t, env)
	orderID := env.createOrder(t, attacker, product)

	res := env.verify(attacker, orderID, product.ID.String(), purchaseToken()) // never registered with Play

	assert.GreaterOrEqual(t, res.Code, 400)
	assert.False(t, env.entitled(t, attacker, product))
}

// One paid token, two orders: the second order must not ride on the first
// purchase, even for the same account.
func TestPurchaseTampering_TokenReplayedAgainstADifferentOrder_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	buyer := env.Register(t)
	first, second := playProduct(t, env), playProduct(t, env)

	firstOrder := env.createOrder(t, buyer, first)
	secondOrder := env.createOrder(t, buyer, second)
	tok := purchaseToken()
	fakePlay.Purchased(tok)
	require.Equal(t, http.StatusOK, env.verify(buyer, firstOrder, first.ID.String(), tok).Code)

	replay := env.verify(buyer, secondOrder, second.ID.String(), tok)

	assert.GreaterOrEqual(t, replay.Code, 400)
	assert.Equal(t, "PENDING", env.orderStatus(t, secondOrder), "the replayed order must not settle")
	assert.False(t, env.entitled(t, buyer, second), "one payment must buy one thing")
}

func TestPurchaseTampering_TokenReusedAcrossUsers_LeavesTheThiefUnentitled(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	buyer, thief := env.Register(t), env.Register(t)
	product := playProduct(t, env)

	buyerOrder := env.createOrder(t, buyer, product)
	tok := purchaseToken()
	fakePlay.Purchased(tok)
	require.Equal(t, http.StatusOK, env.verify(buyer, buyerOrder, product.ID.String(), tok).Code)

	thiefOrder := env.createOrder(t, thief, product)
	res := env.verify(thief, thiefOrder, product.ID.String(), tok)

	assert.GreaterOrEqual(t, res.Code, 400)
	assert.Equal(t, "PENDING", env.orderStatus(t, thiefOrder))
	assert.False(t, env.entitled(t, thief, product))
	assert.True(t, env.entitled(t, buyer, product), "the rightful buyer keeps their access")
}

// Settling someone else's order with a valid token must not work, and must not
// hand the victim's order to the attacker or change its state.
func TestPurchaseTampering_VerifyingAnotherUsersOrder_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	victim, attacker := env.Register(t), env.Register(t)
	product := playProduct(t, env)
	victimOrder := env.createOrder(t, victim, product)
	tok := purchaseToken()
	fakePlay.Purchased(tok)

	res := env.verify(attacker, victimOrder, product.ID.String(), tok)

	assert.GreaterOrEqual(t, res.Code, 400)
	assert.Equal(t, "PENDING", env.orderStatus(t, victimOrder))
	assert.False(t, env.entitled(t, attacker, product))
	assert.False(t, env.entitled(t, victim, product),
		"a stranger's verify call must not settle the victim's order either")
}

func TestPurchaseTampering_CancelledAndPendingPurchases_GrantNothing(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	buyer := env.Register(t)

	for name, register := range map[string]func(string){
		"cancelled": fakePlay.Canceled,
		"pending":   fakePlay.Pending,
	} {
		t.Run(name, func(t *testing.T) {
			product := playProduct(t, env)
			orderID := env.createOrder(t, buyer, product)
			tok := purchaseToken()
			register(tok)

			res := env.verify(buyer, orderID, product.ID.String(), tok)

			assert.GreaterOrEqual(t, res.Code, 400)
			assert.False(t, env.entitled(t, buyer, product))
		})
	}
}

func TestPurchaseTampering_InvalidIdentifiers_AreRejectedWithoutServerErrors(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	buyer := env.Register(t)

	for name, body := range map[string]map[string]string{
		"order id not a uuid":    {"order_id": "1 OR 1=1", "product_id": "x", "purchase_token": "t"},
		"missing purchase token": {"order_id": "00000000-0000-0000-0000-000000000000", "product_id": "x"},
		"empty body":             {},
	} {
		t.Run(name, func(t *testing.T) {
			res := env.POST("/payment/play/verify", buyer.Token, body)
			assert.GreaterOrEqual(t, res.Code, 400)
			assert.Less(t, res.Code, 500, "bad input is the client's error: %s", string(res.Body))
			assert.False(t, strings.Contains(strings.ToLower(string(res.Body)), "sql"))
		})
	}
}
