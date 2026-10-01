package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/tests/fixtures"
)

// Proves the harness itself works: a cloned database exists, the project's
// real migrations have been applied to it, and it is empty of test data.
func TestHarness_ClonedDatabaseHasMigratedSchema(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	for _, table := range []string{
		"parents", "products", "orders", "user_entitlements",
		"premium_packages", "premium_package_items", "user_subscriptions",
		"ar_cards", "dongengs", "email_verification_tokens",
	} {
		var exists bool
		require.NoError(t, db.Raw(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = ?)`,
			table,
		).Scan(&exists).Error)
		assert.True(t, exists, "table %q should exist after migrations", table)
	}
}

// R__init.sql inserts demo parents and AR cards. It is deliberately excluded
// from the template so row counts stay deterministic — if it ever leaks in,
// this fails.
func TestHarness_TemplateContainsNoSeedData(t *testing.T) {
	t.Parallel()
	db := fixtures.FreshDB(t)

	var parents int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM parents`).Scan(&parents).Error)
	assert.Zero(t, parents, "template must not carry demo data from R__init.sql")
}

// Each test gets its own database, so a write in one is invisible to another.
func TestHarness_DatabasesAreIsolated(t *testing.T) {
	t.Parallel()
	a := fixtures.FreshDB(t)
	b := fixtures.FreshDB(t)

	require.NoError(t, a.Exec(
		`INSERT INTO parents (name, phone_number, email_address, password, city)
		 VALUES ('A','081','a@example.com','h','Jakarta')`).Error)

	var countB int64
	require.NoError(t, b.Raw(`SELECT COUNT(*) FROM parents`).Scan(&countB).Error)
	assert.Zero(t, countB, "a write in one test database must not be visible in another")
}
