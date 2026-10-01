package api_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// Refunding Google Play orders from the backoffice, through the real router
// and database, against fixtures.FakePlay standing in for Google.

// adminSession creates an admin user and returns a token for it.
func adminSession(t *testing.T, env *Env) (uuid.UUID, string) {
	t.Helper()
	admin := models.AdminUser{
		Email:        fmt.Sprintf("admin-%d@example.test", fixtures.NextSeq()),
		PasswordHash: "x",
	}
	require.NoError(t, env.DB.Create(&admin).Error)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": admin.ID.String(), "email": admin.Email, "role": "admin",
		"jti": uuid.NewString(), "exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return admin.ID, signed
}

type playPurchase struct {
	account       *Account
	orderID       string
	purchaseToken string
	productID     uuid.UUID // single product (one-time purchases)
}

// buyProductOnPlay settles a one-time Play purchase of a single product.
func buyProductOnPlay(t *testing.T, env *Env) playPurchase {
	t.Helper()
	account := env.Register(t)
	product, _ := playProduct(t, env)
	created := env.POST("/payment/play/create-product", account.Token,
		map[string]string{"product_id": product.ID.String()})
	require.Equal(t, http.StatusOK, created.Code, string(created.Body))
	orderID := created.Data()["order_id"].(string)

	purchaseToken := token(t)
	fakePlay.Purchased(purchaseToken)
	fakePlay.OrderTotal(fixtures.PlayOrderID(purchaseToken), "25000")
	verified := env.POST("/payment/play/verify", account.Token, map[string]string{
		"order_id": orderID, "product_id": product.ID.String(), "purchase_token": purchaseToken,
	})
	require.Equal(t, http.StatusOK, verified.Code, string(verified.Body))
	return playPurchase{account: account, orderID: orderID, purchaseToken: purchaseToken, productID: product.ID}
}

// buySubscriptionOnPlay settles a Play subscription purchase.
func buySubscriptionOnPlay(t *testing.T, env *Env) playPurchase {
	t.Helper()
	account := env.Register(t)
	sku := fmt.Sprintf("sub_%d", fixtures.NextSeq())
	pkg := fixtures.NewPackage(t, env.DB, fixtures.AsSubscription(30), fixtures.WithPackagePlayProductID(sku))
	created := env.POST("/payment/play/create", account.Token, map[string]string{"package_id": pkg.ID})
	require.Equal(t, http.StatusOK, created.Code, string(created.Body))
	orderID := created.Data()["order_id"].(string)

	purchaseToken := token(t)
	fakePlay.SubscriptionActive(purchaseToken, strconv.FormatInt(time.Now().Add(30*24*time.Hour).UnixMilli(), 10))
	verified := env.POST("/payment/play/verify", account.Token, map[string]string{
		"order_id": orderID, "product_id": sku, "purchase_token": purchaseToken,
	})
	require.Equal(t, http.StatusOK, verified.Code, string(verified.Body))
	return playPurchase{account: account, orderID: orderID, purchaseToken: purchaseToken}
}

func orderStatus(t *testing.T, env *Env, orderID string) string {
	t.Helper()
	var order models.Order
	require.NoError(t, env.DB.First(&order, "id = ?", orderID).Error)
	return order.Status
}

func refundsOf(t *testing.T, env *Env, orderID string) []models.OrderRefund {
	t.Helper()
	var refunds []models.OrderRefund
	require.NoError(t, env.DB.Where("order_id = ?", orderID).Find(&refunds).Error)
	return refunds
}

const refundReason = "Pengguna salah beli kartu"

func TestRefund_OneTimePurchase_RefundsRevokesAndRecords(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	adminID, admin := adminSession(t, env)
	p := buyProductOnPlay(t, env)
	userID := uuid.MustParse(p.account.ID)
	has, err := models.HasEntitlement(env.DB, userID, p.productID)
	require.NoError(t, err)
	require.True(t, has)

	res := env.POST("/admin/orders/"+p.orderID+"/refund", admin, map[string]string{"reason": refundReason})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))

	assert.True(t, fakePlay.Refunded(fixtures.PlayOrderID(p.purchaseToken)), "orders.refund must be called")
	assert.Equal(t, models.OrderStatusRefunded, orderStatus(t, env, p.orderID))
	has, err = models.HasEntitlement(env.DB, userID, p.productID)
	require.NoError(t, err)
	assert.False(t, has, "a refunded purchase must no longer unlock the content")

	refunds := refundsOf(t, env, p.orderID)
	require.Len(t, refunds, 1)
	r := refunds[0]
	assert.Equal(t, models.RefundSourceAdmin, r.Source)
	assert.Equal(t, models.RefundStatusSucceeded, r.Status)
	assert.Equal(t, models.RefundTypeFull, r.RefundType)
	assert.Equal(t, adminID, *r.AdminID)
	assert.Equal(t, refundReason, *r.Reason)
	assert.Equal(t, fixtures.PlayOrderID(p.purchaseToken), *r.PlayOrderID)
	assert.EqualValues(t, 25000, r.OrderAmountIdr)
	require.NotNil(t, r.RefundedTotal, "amounts come from orders.get")
	assert.InDelta(t, 25000, *r.RefundedTotal, 0.001)
	assert.Equal(t, "IDR", *r.Currency)
	assert.Equal(t, "REFUNDED", *r.PlayOrderState)
	assert.NotNil(t, r.CompletedAt)

	history := env.GET("/admin/orders/"+p.orderID+"/refunds", admin)
	require.Equal(t, http.StatusOK, history.Code, string(history.Body))
	rows := history.JSON()["data"].([]interface{})
	require.Len(t, rows, 1)
	assert.NotEmpty(t, rows[0].(map[string]interface{})["admin_email"])

	list := env.GET("/admin/orders?search="+p.orderID, admin)
	require.Equal(t, http.StatusOK, list.Code)
	assert.EqualValues(t, 1, list.JSON()["data"].([]interface{})[0].(map[string]interface{})["refund_count"])

	again := env.POST("/admin/orders/"+p.orderID+"/refund", admin, map[string]string{"reason": refundReason})
	assert.Equal(t, http.StatusConflict, again.Code, "a refunded order can't be refunded again")
}

func TestRefund_Subscription_ProratedRevokesTheSubscription(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	p := buySubscriptionOnPlay(t, env)

	res := env.POST("/admin/orders/"+p.orderID+"/refund", admin,
		map[string]string{"reason": refundReason, "refund_type": "PRORATED"})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))

	assert.Equal(t, "proratedRefund", fakePlay.Revoked(p.purchaseToken))
	assert.Equal(t, models.OrderStatusRefunded, orderStatus(t, env, p.orderID))
	var sub models.UserSubscription
	require.NoError(t, env.DB.First(&sub, "user_id = ?", p.account.ID).Error)
	assert.Equal(t, "free", sub.Status, "the subscription ends immediately")
	refunds := refundsOf(t, env, p.orderID)
	require.Len(t, refunds, 1)
	assert.Equal(t, models.RefundTypeProrated, refunds[0].RefundType)
}

func TestRefund_GoogleRefuses_OrderStaysPaidAndFailureIsRecorded(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	p := buyProductOnPlay(t, env)
	fakePlay.RefuseRefund(fixtures.PlayOrderID(p.purchaseToken), http.StatusBadRequest)

	res := env.POST("/admin/orders/"+p.orderID+"/refund", admin, map[string]string{"reason": refundReason})
	assert.Equal(t, http.StatusBadGateway, res.Code, string(res.Body))
	assert.Contains(t, res.JSON()["error"], "refund not allowed", "Google's message is shown to the admin")

	assert.Equal(t, models.OrderStatusPaid, orderStatus(t, env, p.orderID))
	has, err := models.HasEntitlement(env.DB, uuid.MustParse(p.account.ID), p.productID)
	require.NoError(t, err)
	assert.True(t, has, "access is untouched when Google refuses")
	refunds := refundsOf(t, env, p.orderID)
	require.Len(t, refunds, 1)
	assert.Equal(t, models.RefundStatusFailed, refunds[0].Status)
	assert.NotNil(t, refunds[0].Error)
}

func TestRefund_RejectsBadRequestsWithoutCallingGoogle(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	p := buyProductOnPlay(t, env)

	short := env.POST("/admin/orders/"+p.orderID+"/refund", admin, map[string]string{"reason": "salah"})
	assert.Equal(t, http.StatusBadRequest, short.Code, "reason too short")

	prorated := env.POST("/admin/orders/"+p.orderID+"/refund", admin,
		map[string]string{"reason": refundReason, "refund_type": "PRORATED"})
	assert.Equal(t, http.StatusBadRequest, prorated.Code, "prorated is for subscriptions only")

	pending := env.Register(t)
	product, _ := playProduct(t, env)
	created := env.POST("/payment/play/create-product", pending.Token, map[string]string{"product_id": product.ID.String()})
	require.Equal(t, http.StatusOK, created.Code)
	notPaid := env.POST("/admin/orders/"+created.Data()["order_id"].(string)+"/refund", admin,
		map[string]string{"reason": refundReason})
	assert.Equal(t, http.StatusConflict, notPaid.Code, "only PAID orders are refundable")

	assert.False(t, fakePlay.Refunded(fixtures.PlayOrderID(p.purchaseToken)))
	assert.Empty(t, refundsOf(t, env, p.orderID))

	user := env.GET("/admin/orders/"+p.orderID+"/refunds", p.account.Token)
	assert.Equal(t, http.StatusForbidden, user.Code, "a user token can't reach admin refunds")
}

func TestRefund_GoogleInitiatedRefundIsRecorded(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	p := buyProductOnPlay(t, env)
	fakePlay.Void(p.purchaseToken)

	res := env.POST("/admin/orders/reconcile-play", admin, nil)
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))

	assert.Equal(t, models.OrderStatusRefunded, orderStatus(t, env, p.orderID))
	refunds := refundsOf(t, env, p.orderID)
	require.Len(t, refunds, 1)
	assert.Equal(t, models.RefundSourceGoogleVoided, refunds[0].Source)
	assert.Equal(t, models.RefundStatusSucceeded, refunds[0].Status)
	assert.Nil(t, refunds[0].AdminID)
}

func TestRefund_AlreadyRefundedAtGoogle_IsCompletedHere(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	_, admin := adminSession(t, env)
	p := buyProductOnPlay(t, env)
	fakePlay.AlreadyRefunded(fixtures.PlayOrderID(p.purchaseToken))

	res := env.POST("/admin/orders/"+p.orderID+"/refund", admin, map[string]string{"reason": refundReason})
	require.Equal(t, http.StatusOK, res.Code, string(res.Body))

	assert.Equal(t, models.OrderStatusRefunded, orderStatus(t, env, p.orderID))
	refunds := refundsOf(t, env, p.orderID)
	require.Len(t, refunds, 1)
	assert.Equal(t, models.RefundStatusSucceeded, refunds[0].Status)
	require.NotNil(t, refunds[0].Error)
	assert.Contains(t, *refunds[0].Error, "already refunded at Google")
}
