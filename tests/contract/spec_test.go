package contract_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/registry"
	"arunika_backend/routes"
	"arunika_backend/tests/fixtures"
)

// openapi.yaml is the contract between this backend and its two clients. It
// is only worth anything if it cannot drift from the router, so these tests
// compare the two in BOTH directions:
//
//   - a route added without a spec entry fails here, so the clients find out
//     at review time rather than at runtime;
//   - a spec entry with no route fails here too, which is the case that
//     actually bit this codebase — the Flutter app calls /animals and
//     /fairy-tales/popular, neither of which the router serves.

var ginParam = regexp.MustCompile(`:([a-zA-Z_]+)`)

// ginToOAS converts /orders/:id to /orders/{id}.
func ginToOAS(p string) string { return ginParam.ReplaceAllString(p, "{$1}") }

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	path := specPath(t)
	doc, err := openapi3.NewLoader().LoadFromFile(path)
	require.NoError(t, err, "openapi.yaml must parse")
	require.NoError(t, doc.Validate(t.Context()), "openapi.yaml must be a valid OpenAPI document")
	return doc
}

func specPath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "openapi.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate openapi.yaml")
	return ""
}

// routerOperations returns "METHOD /path" for every registered route, with
// gin parameters rewritten to OpenAPI style.
func routerOperations(t *testing.T) map[string]bool {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := fixtures.FreshDB(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	r := routes.SetupRouter(registry.NewServiceRegistry(db, rdb), rdb, db)

	ops := map[string]bool{}
	for _, ri := range r.Routes() {
		ops[ri.Method+" "+ginToOAS(ri.Path)] = true
	}
	return ops
}

func specOperations(t *testing.T, doc *openapi3.T) map[string]bool {
	t.Helper()
	ops := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			ops[method+" "+path] = true
		}
	}
	return ops
}

func TestSpec_IsValidOpenAPI(t *testing.T) {
	loadSpec(t)
}

func TestSpec_EveryRouteIsDocumented(t *testing.T) {
	doc := loadSpec(t)
	spec := specOperations(t, doc)

	var missing []string
	for op := range routerOperations(t) {
		if !spec[op] {
			missing = append(missing, op)
		}
	}
	sort.Strings(missing)

	assert.Empty(t, missing,
		"these routes exist but are absent from openapi.yaml — add them, or the "+
			"clients have no way to know they exist:\n  %s", strings.Join(missing, "\n  "))
}

func TestSpec_EveryDocumentedOperationExists(t *testing.T) {
	doc := loadSpec(t)
	router := routerOperations(t)

	var phantom []string
	for op := range specOperations(t, doc) {
		if !router[op] {
			phantom = append(phantom, op)
		}
	}
	sort.Strings(phantom)

	assert.Empty(t, phantom,
		"openapi.yaml documents operations the router does not serve — a client "+
			"trusting the spec would get 404s:\n  %s", strings.Join(phantom, "\n  "))
}

// Security is the half of the contract most likely to rot silently: adding a
// route to an authenticated group is easy to do without touching the spec.
// These assert the documented scheme matches how the router actually behaves.
func TestSpec_AuthenticatedOperationsDeclareASecurityScheme(t *testing.T) {
	doc := loadSpec(t)

	var undeclared []string
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			// An operation must state its stance explicitly — either a scheme
			// or an empty security list marking it public. A missing security
			// key silently inherits the document default, which is how an
			// admin route ends up looking public.
			if op.Security == nil {
				undeclared = append(undeclared, method+" "+path)
			}
		}
	}
	sort.Strings(undeclared)

	assert.Empty(t, undeclared,
		"these operations declare no security stance — set `security: []` for "+
			"public routes:\n  %s", strings.Join(undeclared, "\n  "))
}

func TestSpec_AdminRoutesRequireAdminAuth(t *testing.T) {
	doc := loadSpec(t)

	var wrong []string
	for path, item := range doc.Paths.Map() {
		if !strings.HasPrefix(path, "/admin/") {
			continue
		}
		// The two admin login endpoints necessarily precede having a token.
		if path == "/admin/auth/login" || path == "/admin/auth/refresh" {
			continue
		}
		for method, op := range item.Operations() {
			declared := ""
			if op.Security != nil && len(*op.Security) > 0 {
				for name := range (*op.Security)[0] {
					declared = name
				}
			}
			if declared != "adminAuth" {
				wrong = append(wrong, method+" "+path+" (declares "+declared+")")
			}
		}
	}
	sort.Strings(wrong)

	assert.Empty(t, wrong,
		"every /admin route must require adminAuth:\n  %s", strings.Join(wrong, "\n  "))
}
