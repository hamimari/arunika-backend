package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"arunika_backend/tests/fixtures"
)

// Migrations are the schema's source of truth, and nothing exercised them
// from empty until now. A fresh environment — a new developer, a new staging
// database, CI — applies them in one go, so a migration that only works
// against an already-populated database is broken for everyone but the
// person who wrote it.
//
// This is the test that caught V49 ending in a stray "®" (bytes c2 ae),
// which made the entire set fail partway through.

func TestMigrations_ApplyCleanlyFromEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := fixtures.EmptyDB(t)

	require.NoError(t, fixtures.ApplyVersionedMigrations(ctx, db),
		"every versioned migration must apply to an empty database")

	// Spot-check that the tables the product actually depends on exist,
	// rather than trusting that no error means the right schema.
	for _, table := range []string{
		"parents", "children", "products", "features",
		"orders", "payments", "user_entitlements", "user_subscriptions",
		"premium_packages", "premium_package_items",
		"ar_cards", "dongengs", "email_verification_tokens",
	} {
		var exists bool
		require.NoError(t, db.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`,
			table,
		).Scan(&exists))
		assert.True(t, exists, "migrations must create %q", table)
	}
}

func TestMigrations_AreNumberedWithoutGapsOrDuplicates(t *testing.T) {
	t.Parallel()

	paths, err := fixtures.VersionedMigrationFiles()
	require.NoError(t, err)
	require.NotEmpty(t, paths)

	// Flyway applies by version, so a duplicate version is ambiguous and a
	// gap usually means a migration was lost in a merge.
	seen := map[int]string{}
	for i, p := range paths {
		version := i + 1
		if prev, dup := seen[version]; dup {
			t.Fatalf("duplicate migration version %d: %s and %s", version, prev, p)
		}
		seen[version] = p
	}
	assert.Len(t, seen, len(paths))
}

// Repeatable migrations re-run on every Flyway invocation, so applying them
// twice must leave the same state. R__init.sql TRUNCATEs and re-seeds demo
// data, which is exactly why it is kept out of the shared template.
func TestMigrations_RepeatablesAreIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := fixtures.EmptyDB(t)

	require.NoError(t, fixtures.ApplyVersionedMigrations(ctx, db))
	require.NoError(t, fixtures.ApplyRepeatableMigrations(ctx, db),
		"repeatable migrations must apply after the versioned set")

	countParents := func() int {
		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM parents`).Scan(&n))
		return n
	}
	afterFirst := countParents()

	require.NoError(t, fixtures.ApplyRepeatableMigrations(ctx, db),
		"repeatable migrations must survive a second run")

	assert.Equal(t, afterFirst, countParents(),
		"re-running repeatables must not duplicate seed rows")
}

// V55 grandfathers accounts that predate email verification. Re-applying it
// must never re-grandfather: that would mark genuinely unverified accounts
// as verified, handing password recovery to addresses nobody has proven they
// control. Flyway does not re-run versioned migrations, but this must not
// depend on that — test harnesses and manual recovery steps do.
func TestMigrations_V55_ReapplyDoesNotVerifyNewAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := fixtures.EmptyDB(t)

	require.NoError(t, fixtures.ApplyVersionedMigrations(ctx, db))

	_, err := db.ExecContext(ctx,
		`INSERT INTO parents (name, phone_number, email_address, password, city, email_verified)
		 VALUES ('New', '0899', 'new@example.test', 'h', 'Jakarta', false)`)
	require.NoError(t, err)

	paths, err := fixtures.VersionedMigrationFiles()
	require.NoError(t, err)
	v55 := paths[len(paths)-1]
	require.Contains(t, v55, "V55__", "expected V55 to be the newest migration")

	require.NoError(t, fixtures.ApplyFile(ctx, db, v55), "V55 must be safely re-runnable")

	var verified bool
	require.NoError(t, db.QueryRow(
		`SELECT email_verified FROM parents WHERE email_address = 'new@example.test'`,
	).Scan(&verified))
	assert.False(t, verified,
		"re-running V55 must not grandfather an account created after it first ran")
}

// Regression: the seeded admin password hash did not actually match its
// documented password. Nothing had ever verified the two agree, so a fresh
// environment's admin login silently failed with "invalid credentials"
// against a genuinely correct-looking seed file — found while wiring up the
// Flutter integration_test harness, which needs a real admin session to seed
// content for its flows.
func TestSeeds_AdminUserPasswordMatchesItsDocumentedValue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := fixtures.EmptyDB(t)

	require.NoError(t, fixtures.ApplyVersionedMigrations(ctx, db))
	require.NoError(t, fixtures.ApplyRepeatableMigrations(ctx, db))

	var hash string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT password_hash FROM admin_users WHERE email = 'admin@arunika.id'`,
	).Scan(&hash))

	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin123")),
		"the seeded admin_users row must actually verify against the password documented in R__seed_admin_user.sql")
}
