package security_test

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// ── 7.2 Horizontal escalation: user A reaching user B's data ────────────────

func TestHorizontal_OrderOfAnotherUser_IsNotVisible(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	a, b := env.Register(t), env.Register(t)

	var owner models.Parent
	require.NoError(t, env.DB.First(&owner, "id = ?", b.ID).Error)
	product := fixtures.NewProduct(t, env.DB)
	order := fixtures.NewOrderForProduct(t, env.DB, &owner, product)

	assert.Equal(t, http.StatusOK, env.GET("/orders/"+order.ID.String(), b.Token).Code, "sanity: the owner can read it")

	res := env.GET("/orders/"+order.ID.String(), a.Token)
	assert.Equal(t, http.StatusNotFound, res.Code,
		"another user's order must look exactly like a missing one")
	assert.NotContains(t, string(res.Body), owner.EmailAddress)
}

func TestHorizontal_OrderListOfAnotherUser_IsNotVisible(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	a, b := env.Register(t), env.Register(t)

	var owner models.Parent
	require.NoError(t, env.DB.First(&owner, "id = ?", b.ID).Error)
	order := fixtures.NewOrderForProduct(t, env.DB, &owner, fixtures.NewProduct(t, env.DB))

	res := env.GET("/orders", a.Token)
	require.Equal(t, http.StatusOK, res.Code)
	assert.NotContains(t, string(res.Body), order.ID.String())
}

func TestHorizontal_NotificationsOfAnotherUser_CannotBeReadOrMarkedRead(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	a, b := env.Register(t), env.Register(t)

	note := models.Notification{UserID: b.ID, Title: "private title", Body: "private body", Type: "payment"}
	require.NoError(t, env.DB.Create(&note).Error)

	list := env.GET("/notifications", a.Token)
	require.Equal(t, http.StatusOK, list.Code)
	assert.NotContains(t, string(list.Body), "private title", "A's inbox must not include B's notification")

	mark := env.PATCH("/notifications/"+note.ID.String()+"/read", a.Token, nil)
	assert.Equal(t, http.StatusNotFound, mark.Code)

	var after models.Notification
	require.NoError(t, env.DB.First(&after, "id = ?", note.ID).Error)
	assert.False(t, after.IsRead, "A must not be able to change B's notification state")
}

func TestHorizontal_FairyTaleHistoryOfAnotherUser_IsNotVisible(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	a, b := env.Register(t), env.Register(t)
	story := fixtures.NewDongeng(t, env.DB, fixtures.WithTitle("B's private listening history"))
	require.NoError(t, models.RecordDongengPlay(env.DB, b.ID, story.ID))

	own := env.GET("/fairy-tales/history", b.Token)
	require.Equal(t, http.StatusOK, own.Code)
	require.Contains(t, string(own.Body), story.ID.String(), "sanity: the owner sees their history")

	other := env.GET("/fairy-tales/history", a.Token)
	require.Equal(t, http.StatusOK, other.Code)
	assert.NotContains(t, string(other.Body), story.ID.String())
}

func TestHorizontal_UserProfileOfAnotherUser_IsForbidden(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	a, b := env.Register(t), env.Register(t)

	res := env.GET("/user/"+b.ID.String(), a.Token)

	assert.Equal(t, http.StatusForbidden, res.Code)
	assert.NotContains(t, string(res.Body), b.Email)
}

// Growth measurements hang off a child, and the child belongs to a parent, so
// every /children/:childId/growth route must check that the child is the
// caller's and that the measurement is that child's. A foreign child or
// measurement must look exactly like a missing one.

// growthFixture gives b's child a usable profile and one measurement.
func growthFixture(t *testing.T, env *Env, b *Account) (child uuid.UUID, m models.GrowthMeasurement) {
	t.Helper()
	child = env.ChildID(t, b)
	require.NoError(t, env.DB.Model(&models.Children{}).Where("id = ?", child).
		Updates(map[string]interface{}{"gender": "Laki-Laki", "date_of_birth": time.Now().AddDate(-3, 0, 0)}).Error)
	w := 15.5
	m = models.GrowthMeasurement{ChildID: child, MeasuredOn: time.Now().AddDate(0, -1, 0), WeightKg: &w, Position: "standing", AgeDays: 1065}
	require.NoError(t, env.DB.Create(&m).Error)
	return child, m
}

func TestHorizontal_GrowthOfAnotherUsersChild_CannotBeRead(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	a, b := env.Register(t), env.Register(t)
	child, m := growthFixture(t, env, b)

	own := env.GET("/children/"+child.String()+"/growth", b.Token)
	require.Equal(t, http.StatusOK, own.Code)
	require.Contains(t, string(own.Body), m.ID.String(), "sanity: the parent sees their child's measurements")

	res := env.GET("/children/"+child.String()+"/growth", a.Token)
	assert.Equal(t, http.StatusNotFound, res.Code)
	assert.NotContains(t, string(res.Body), m.ID.String())

	unknown := env.GET("/children/"+uuid.NewString()+"/growth", a.Token)
	assert.Equal(t, res.Code, unknown.Code, "a foreign child and a nonexistent one must be indistinguishable")
	assert.JSONEq(t, string(res.Body), string(unknown.Body))
}

func TestHorizontal_GrowthOfAnotherUsersChild_CannotBeCreated(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	a, b := env.Register(t), env.Register(t)
	child, _ := growthFixture(t, env, b)

	res := env.POST("/children/"+child.String()+"/growth/measurements", a.Token, map[string]interface{}{
		"measured_on": time.Now().Format("2006-01-02"), "weight_kg": 2, "position": "standing",
	})

	assert.Equal(t, http.StatusNotFound, res.Code)
	var count int64
	env.DB.Model(&models.GrowthMeasurement{}).Where("child_id = ?", child).Count(&count)
	assert.Equal(t, int64(1), count, "nothing may be written to another user's child")
}

func TestHorizontal_GrowthOfAnotherUsersChild_CannotBeEditedDeletedOrRestored(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	a, b := env.Register(t), env.Register(t)
	child, m := growthFixture(t, env, b)
	ownChild := env.ChildID(t, a)
	base := "/children/" + child.String() + "/growth/measurements/" + m.ID.String()
	// The attacker's own child id with the victim's measurement id.
	mixed := "/children/" + ownChild.String() + "/growth/measurements/" + m.ID.String()

	for _, path := range []string{base, mixed} {
		assert.Equal(t, http.StatusNotFound, env.PATCH(path, a.Token, map[string]float64{"weight_kg": 1}).Code, path)
		assert.Equal(t, http.StatusNotFound, env.Do(http.MethodDelete, path, a.Token, nil).Code, path)
	}
	var after models.GrowthMeasurement
	require.NoError(t, env.DB.First(&after, "id = ?", m.ID).Error)
	assert.Equal(t, 15.5, *after.WeightKg, "the measurement must be unchanged")
	assert.Nil(t, after.DeletedAt, "the measurement must not be deleted")

	// Once the owner deletes it, the attacker cannot restore it either.
	require.Equal(t, http.StatusNoContent, env.Do(http.MethodDelete, base, b.Token, nil).Code)
	for _, path := range []string{base, mixed} {
		assert.Equal(t, http.StatusNotFound, env.POST(path+"/restore", a.Token, nil).Code, path)
	}
	require.NoError(t, env.DB.First(&after, "id = ?", m.ID).Error)
	assert.NotNil(t, after.DeletedAt, "the measurement must stay deleted")
}

func TestGrowth_OwnerCanReadCreateEditDeleteAndRestore(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	parent := env.Register(t)
	child, _ := growthFixture(t, env, parent)
	base := "/children/" + child.String() + "/growth"

	created := env.POST(base+"/measurements", parent.Token, map[string]interface{}{
		"measured_on": time.Now().AddDate(0, 0, -1).Format("2006-01-02"), "weight_kg": 14, "position": "standing",
	})
	require.Equal(t, http.StatusCreated, created.Code, string(created.Body))
	id := created.Data()["id"].(string)

	assert.Equal(t, http.StatusOK, env.PATCH(base+"/measurements/"+id, parent.Token, map[string]float64{"weight_kg": 14.2}).Code)
	assert.Equal(t, http.StatusNoContent, env.Do(http.MethodDelete, base+"/measurements/"+id, parent.Token, nil).Code)
	assert.Equal(t, http.StatusOK, env.POST(base+"/measurements/"+id+"/restore", parent.Token, nil).Code)

	list := env.GET(base, parent.Token)
	require.Equal(t, http.StatusOK, list.Code)
	assert.Contains(t, string(list.Body), id)
}

// ── 7.3 Vertical escalation: a user token on every admin route ──────────────

var routeParam = regexp.MustCompile(`:[A-Za-z_]+`)

// Iterating the live route table means a newly added /admin route is covered
// the day it is registered, with no test to remember to write.
func TestVertical_EveryAdminRoute_RejectsAnOrdinaryUser(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)

	// The only /admin routes that are meant to be reachable without admin rights.
	public := map[string]bool{
		"POST /admin/auth/login":   true,
		"POST /admin/auth/refresh": true,
	}

	checked := 0
	for _, route := range env.Router.Routes() {
		if !strings.HasPrefix(route.Path, "/admin") || public[route.Method+" "+route.Path] {
			continue
		}
		path := routeParam.ReplaceAllString(route.Path, uuid.NewString())
		name := route.Method + " " + route.Path
		checked++

		asUser := env.send(route.Method, path, user.Token, map[string]string{})
		assert.Equal(t, http.StatusForbidden, asUser.Code, "%s with a user token: %s", name, string(asUser.Body))

		anonymous := env.send(route.Method, path, "", map[string]string{})
		assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, anonymous.Code,
			"%s with no token", name)
	}

	// Guards the guard: if route discovery ever breaks, this must not pass vacuously.
	require.Greater(t, checked, 50, "found suspiciously few admin routes — is the route table being read?")
	t.Logf("checked %d admin routes", checked)
}

func TestVertical_UserTokenWithForgedAdminRole_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)

	// Same claims a real admin token carries, but signed with the wrong key.
	forged := adminToken(t, user.ID, []byte("an-attacker-chosen-secret-key-0123456789"))

	res := env.GET("/admin/users", forged)

	assert.Equal(t, http.StatusUnauthorized, res.Code)
}

func TestVertical_AdminToken_IsAcceptedOnAdminRoutes(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)

	// Sanity: the 403s above are about the missing role, not a broken route.
	res := env.GET("/admin/users", adminToken(t, uuid.New(), []byte(testSecret)))

	assert.Equal(t, http.StatusOK, res.Code, "body: %s", string(res.Body))
}

func TestVertical_AdminToken_DoesNotOpenUserRoutesAsAnotherIdentity(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	victim := env.Register(t)

	res := env.GET("/user/"+victim.ID.String(), adminToken(t, uuid.New(), []byte(testSecret)))

	assert.NotEqual(t, http.StatusOK, res.Code,
		"an admin session must not double as access to a user's private profile")
	assert.NotContains(t, string(res.Body), fmt.Sprint(victim.Email))
}
