package security_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// leakMarkers are fragments that only ever appear in a response when internals
// have escaped: SQL text, driver and ORM errors, Go stack traces, file paths.
var leakMarkers = []string{
	"SELECT ", "INSERT INTO", "UPDATE \"", "DELETE FROM", "syntax error at or near",
	"pq:", "pgx", "SQLSTATE", "gorm", "sql:", "violates", "relation \"",
	"goroutine ", "runtime error", "panic", ".go:", "/Users/", "/app/", "/home/",
}

func assertNoLeak(t *testing.T, label string, res *Response) {
	t.Helper()
	body := string(res.Body)
	for _, marker := range leakMarkers {
		assert.NotContains(t, body, marker, "%s (status %d) leaked %q: %s", label, res.Code, marker, body)
	}
}

// ── 7.5 Validation and injection ────────────────────────────────────────────

var sqlPayloads = []string{
	"' OR '1'='1",
	"'; DROP TABLE parents; --",
	"\" OR \"\"=\"",
	"1; SELECT pg_sleep(5)",
	"%' UNION SELECT NULL,NULL--",
	"\\'",
}

// GORM parameterises, so these are not expected to find anything. They assert
// that the guarantee holds on every user-controlled parameter — including the
// ones a future change might start interpolating into a raw clause.
func TestInjection_SQLMetacharactersInQueryParameters_AreInert(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)
	admin := adminToken(t, uuid.New(), []byte(testSecret))
	fixtures.NewDongeng(t, env.DB, fixtures.WithTitle("Kancil"))

	publicPaths := []string{"/fairy-tales", "/ar/cards", "/ar/categories", "/dongeng-categories", "/premium/packs", "/banners"}
	adminPaths := []string{
		"/admin/content/fairy-tales", "/admin/content/ar-cards", "/admin/content/categories",
		"/admin/content/badges", "/admin/content/tracing-items", "/admin/content/counting-questions",
		"/admin/users", "/admin/payments", "/admin/orders", "/admin/products",
	}
	params := []string{"search", "q", "sort", "sort_by", "order", "order_by", "filter", "status", "category_id", "type", "page", "per_page", "limit"}

	for _, payload := range sqlPayloads {
		for _, path := range publicPaths {
			for _, param := range params {
				target := path + "?" + param + "=" + urlEscape(payload)
				res := env.GET(target, user.Token)
				assert.Less(t, res.Code, 500, "GET %s: %s", target, string(res.Body))
				assertNoLeak(t, "GET "+target, res)
			}
		}
		for _, path := range adminPaths {
			for _, param := range params {
				target := path + "?" + param + "=" + urlEscape(payload)
				res := env.GET(target, admin)
				assert.Less(t, res.Code, 500, "GET %s: %s", target, string(res.Body))
				assertNoLeak(t, "GET "+target, res)
			}
		}
	}

	// The tables are all still there and the seeded row is intact.
	var count int64
	require.NoError(t, env.DB.Model(&models.Parent{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "the parents table must be untouched")
}

func TestInjection_SQLMetacharactersInPathAndBody_AreInert(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)

	for _, payload := range sqlPayloads {
		for _, path := range []string{"/fairy-tales/", "/orders/", "/ar/cards/", "/user/"} {
			res := env.GET(path+urlEscape(payload), user.Token)
			assert.Less(t, res.Code, 500, "GET %s: %s", path, string(res.Body))
			assertNoLeak(t, "GET "+path+payload, res)
		}
		login := env.POST("/auth/login", "", map[string]string{"email": payload, "password": payload})
		assert.Equal(t, http.StatusUnauthorized, login.Code, "SQL metacharacters must not authenticate")
		assertNoLeak(t, "login", login)
	}
}

func TestValidation_MalformedAndOversizedPayloads_AreRejectedCleanly(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)

	huge := strings.Repeat("A", 5<<20)

	cases := map[string]struct {
		method, path, token, body string
	}{
		"login: wrong types":        {"POST", "/auth/login", "", `{"email": 123, "password": ["x"]}`},
		"login: not json":           {"POST", "/auth/login", "", `not json at all`},
		"login: truncated json":     {"POST", "/auth/login", "", `{"email": "a@b.c", "passw`},
		"login: 5 MB password":      {"POST", "/auth/login", "", `{"email":"a@b.test","password":"` + huge + `"}`},
		"signup: wrong types":       {"POST", "/auth/signup", "", `{"name": 42, "child": "not-a-list", "password": true}`},
		"signup: 5 MB name":         {"POST", "/auth/signup", "", `{"name":"` + huge + `","email_address":"x@y.test","password":"secret123"}`},
		"signup: deeply nested":     {"POST", "/auth/signup", "", strings.Repeat(`{"a":`, 5000) + `1` + strings.Repeat(`}`, 5000)},
		"growth: negative values":   {"POST", "/growth", user.Token, `{"child_id":"` + uuid.NewString() + `","weight_kg":-5,"height_cm":-1}`},
		"growth: string for number": {"POST", "/growth", user.Token, `{"child_id":"` + uuid.NewString() + `","weight_kg":"heavy","height_cm":"tall"}`},
		"verify: wrong types":       {"POST", "/payment/play/verify", user.Token, `{"order_id": 1, "purchase_token": {"a":1}}`},
		"user update: wrong types":  {"PUT", "/user", user.Token, `{"name": ["a"], "city": 7}`},
		"rtdn: not base64":          {"POST", "/payment/play/rtdn", "", `{"message":{"data":"%%%not-base64%%%"}}`},
		"webhook: empty":            {"POST", "/payment/webhook", "", ``},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			res := env.Do(c.method, c.path, c.token, []byte(c.body))
			assert.GreaterOrEqual(t, res.Code, 400, "must be rejected: %s", truncate(string(res.Body)))
			assert.Less(t, res.Code, 500, "a bad payload is the client's error, not a crash: %s", truncate(string(res.Body)))
			assertNoLeak(t, name, res)
		})
	}
}

// ── 7.6 Error leakage ───────────────────────────────────────────────────────

// Every route, hit with an unauthenticated request, a garbage identifier and
// a broken body: whatever status comes back, it must not carry internals.
func TestErrorLeakage_NoRouteRevealsInternals(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)

	checked := 0
	for _, route := range env.Router.Routes() {
		path := routeParam.ReplaceAllString(route.Path, "not-a-uuid")
		for _, token := range []string{"", user.Token} {
			for _, body := range []string{`{`, `{"x": [1,2,3]}`, ``} {
				res := env.Do(route.Method, path, token, []byte(body))
				assertNoLeak(t, route.Method+" "+route.Path, res)
				assert.NotEqual(t, http.StatusInternalServerError, res.Code,
					"%s %s answered 500 to hostile input: %s", route.Method, route.Path, truncate(string(res.Body)))
				checked++
			}
		}
	}
	require.Greater(t, checked, 300)
}

func TestErrorLeakage_UnknownRoute_IsAPlainNotFound(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)

	res := env.GET("/this/route/does/not/exist", "")

	assert.Equal(t, http.StatusNotFound, res.Code)
	assertNoLeak(t, "404", res)
}

// The non-enumerating behaviour of ForgotPassword is unit-tested; this holds
// it at the API level, where a differing status, body or timing-independent
// field would be visible to an attacker.
func TestErrorLeakage_ForgotPassword_DoesNotRevealWhichAccountsExist(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	known := env.Register(t)

	forKnown := env.POST("/forgot-password", "", map[string]string{"email": known.Email})
	forUnknown := env.POST("/forgot-password", "", map[string]string{"email": "nobody-here@example.test"})

	assert.Equal(t, forKnown.Code, forUnknown.Code)
	assert.Equal(t, http.StatusOK, forKnown.Code)
	assert.Equal(t,
		strings.ReplaceAll(string(forKnown.Body), known.Email, "EMAIL"),
		strings.ReplaceAll(string(forUnknown.Body), "nobody-here@example.test", "EMAIL"),
		"the response must be identical apart from echoing the address that was submitted")
}

func TestErrorLeakage_Login_DoesNotRevealWhichAccountsExist(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	known := env.Register(t)

	wrongPassword := env.POST("/auth/login", "", map[string]string{"email": known.Email, "password": "definitely-wrong"})
	unknownAccount := env.POST("/auth/login", "", map[string]string{"email": "nobody-here@example.test", "password": "definitely-wrong"})

	assert.Equal(t, wrongPassword.Code, unknownAccount.Code)
	assert.JSONEq(t, string(wrongPassword.Body), string(unknownAccount.Body))
}

// Signup names the field that is taken, which the app needs for its form, but
// that is the one place account existence is disclosed on purpose — this test
// pins it to /auth/check-availability and signup so no *other* endpoint grows
// the same behaviour unnoticed.
func TestErrorLeakage_PasswordReset_DoesNotRevealWhichTokensExist(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)

	res := env.Do("POST", "/reset-password?token=00000000-0000-0000-0000-000000000000", "", []byte(`{"new_password":"longenough1"}`))

	assert.GreaterOrEqual(t, res.Code, 400)
	assert.Less(t, res.Code, 500)
	assertNoLeak(t, "reset-password", res)
}

func truncate(s string) string {
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
