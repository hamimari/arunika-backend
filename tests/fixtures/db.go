package fixtures

import (
	"database/sql"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// FreshDB returns a GORM handle to a brand-new database cloned from the
// migrated template. Use it when the code under test manages its own
// transactions — an entitlement grant, a payment settlement, anything that
// calls Begin/Commit itself — because a test-owned wrapping transaction would
// interfere with those.
//
// Safe to call from tests marked t.Parallel(): each caller gets its own
// database, so concurrent writes to the same table cannot collide.
func FreshDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := newClonedDatabase(t)

	db, err := gorm.Open(
		postgres.Open(dsnFor(name)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("open cloned database %s: %v", name, err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// TxDB returns a GORM handle bound to a transaction that is rolled back when
// the test finishes, so the test leaves no rows behind. It is the cheaper
// option for plain repository reads and writes.
//
// Do not use it for code that commits its own transaction — GORM nests those
// as savepoints, and a test asserting "the outer transaction rolled back"
// would be asserting against savepoint semantics rather than the real thing.
// Reach for FreshDB there.
func TxDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := FreshDB(t)

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin test transaction: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

// EmptyDB returns a raw handle to a brand-new database with no migrations
// applied. It exists for tests about the migration set itself — everything
// else should use FreshDB, which hands back an already-migrated schema.
func EmptyDB(t *testing.T) *sql.DB {
	t.Helper()
	name := newEmptyDatabase(t)

	db, err := sql.Open("pgx", dsnFor(name))
	if err != nil {
		t.Fatalf("open empty database %s: %v", name, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
