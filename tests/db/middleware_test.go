package db_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gorm.io/gorm"

	"arunika_backend/middlewares"
	"arunika_backend/tests/fixtures"
)

func init() { gin.SetMode(gin.TestMode) }

// SubscriptionMiddleware decides whether a user may reach premium-only
// endpoints. It reads user_subscriptions directly, so it is only meaningfully
// testable against a real database — a mock would just replay whatever row
// the test told it to.

func newSubscriptionRouter(t *testing.T) (*gin.Engine, *gorm.DB, func(userID string)) {
	t.Helper()
	db := fixtures.FreshDB(t)

	var actingUserID string
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if actingUserID != "" {
			c.Set("userID", actingUserID)
		}
		c.Next()
	})
	r.GET("/premium", middlewares.SubscriptionMiddleware(db), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	return r, db, func(userID string) { actingUserID = userID }
}

func callPremium(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/premium", nil))
	return w
}

func TestSubscriptionMiddleware_ActiveSubscriber_IsAllowed(t *testing.T) {
	t.Parallel()
	r, db, act := newSubscriptionRouter(t)

	user := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, user)
	act(user.ID.String())

	assert.Equal(t, http.StatusOK, callPremium(r).Code)
}

func TestSubscriptionMiddleware_NoSubscriptionRow_IsForbidden(t *testing.T) {
	t.Parallel()
	r, db, act := newSubscriptionRouter(t)

	user := fixtures.NewUser(t, db)
	act(user.ID.String())

	// A user who never subscribed has no row at all — that is free tier.
	assert.Equal(t, http.StatusForbidden, callPremium(r).Code)
}

func TestSubscriptionMiddleware_FreeTierRow_IsForbidden(t *testing.T) {
	t.Parallel()
	r, db, act := newSubscriptionRouter(t)

	user := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, user, fixtures.WithSubscriptionStatus("free"))
	act(user.ID.String())

	assert.Equal(t, http.StatusForbidden, callPremium(r).Code)
}

// The case that actually costs money if it regresses: a subscription whose
// paid period has ended must stop granting access.
func TestSubscriptionMiddleware_ExpiredSubscription_IsForbidden(t *testing.T) {
	t.Parallel()
	r, db, act := newSubscriptionRouter(t)

	user := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, user, fixtures.ExpiringAt(time.Now().Add(-time.Hour)))
	act(user.ID.String())

	assert.Equal(t, http.StatusForbidden, callPremium(r).Code,
		"a lapsed subscription must not keep unlocking premium endpoints")
}

// A revoked subscription is written as status='free' with expires_at=now
// (see EntitlementService.RevokeSubscription), so it must be refused the
// moment the refund is processed.
func TestSubscriptionMiddleware_RevokedSubscription_IsForbidden(t *testing.T) {
	t.Parallel()
	r, db, act := newSubscriptionRouter(t)

	user := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, user,
		fixtures.WithSubscriptionStatus("free"),
		fixtures.ExpiringAt(time.Now()))
	act(user.ID.String())

	assert.Equal(t, http.StatusForbidden, callPremium(r).Code)
}

func TestSubscriptionMiddleware_Unauthenticated_IsUnauthorized(t *testing.T) {
	t.Parallel()
	r, _, _ := newSubscriptionRouter(t)

	// No upstream auth middleware ran, so no userID is set.
	assert.Equal(t, http.StatusUnauthorized, callPremium(r).Code)
}

func TestSubscriptionMiddleware_MalformedUserID_IsBadRequest(t *testing.T) {
	t.Parallel()
	r, _, act := newSubscriptionRouter(t)

	act("not-a-uuid")

	assert.Equal(t, http.StatusBadRequest, callPremium(r).Code)
}

// One user's subscription must never let another through.
func TestSubscriptionMiddleware_OtherUsersSubscription_DoesNotGrantAccess(t *testing.T) {
	t.Parallel()
	r, db, act := newSubscriptionRouter(t)

	subscriber := fixtures.NewUser(t, db)
	fixtures.NewSubscription(t, db, subscriber)
	bystander := fixtures.NewUser(t, db)

	act(bystander.ID.String())

	assert.Equal(t, http.StatusForbidden, callPremium(r).Code)

	act(subscriber.ID.String())
	require.Equal(t, http.StatusOK, callPremium(r).Code, "sanity: the subscriber still gets in")
}
