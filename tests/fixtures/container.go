// Package fixtures provides the shared test harness for database-backed
// tests: a real PostgreSQL container, the project's own migrations applied to
// it, per-test isolation, and builders for domain objects.
//
// Why a real database rather than sqlmock: the guarantees that matter most in
// this codebase — that the same purchase token cannot grant two entitlements,
// that an order references a real user, that a partial grant rolls back — are
// enforced by PostgreSQL constraints. Asserting them against a mocked driver
// tests the mock's expectation list, not the constraint.
package fixtures

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	dbUser = "arunika_test"
	dbPass = "arunika_test"
	// templateDB holds the migrated schema. Each test clones from it with
	// CREATE DATABASE ... TEMPLATE, which is a file copy inside PostgreSQL and
	// takes milliseconds — far cheaper than re-running 54 migrations per test.
	templateDB = "arunika_template"
)

var (
	container  *postgres.PostgresContainer
	adminDSN   string
	startOnce  sync.Once
	startErr   error
	cloneCount int
	cloneMu    sync.Mutex
)

// versionedMigration matches V<n>__name.sql. Repeatable (R__) migrations are
// deliberately excluded from the template — R__init.sql TRUNCATEs tables and
// inserts demo parents, children and AR cards, which would make row counts
// non-deterministic in every test. Repeatables are exercised on their own
// database by TestMigrations_RepeatablesAreIdempotent instead.
var versionedMigration = regexp.MustCompile(`^V(\d+)__.*\.sql$`)

// StartPostgres boots one PostgreSQL container for the test binary and
// prepares the migrated template database. It is safe to call from multiple
// TestMain functions; the container starts once per process.
func StartPostgres(ctx context.Context) error {
	startOnce.Do(func() {
		startErr = start(ctx)
	})
	return startErr
}

func start(ctx context.Context) error {
	c, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		return fmt.Errorf("start postgres container: %w", err)
	}
	container = c

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("connection string: %w", err)
	}
	adminDSN = dsn

	return buildTemplate(ctx)
}

// buildTemplate creates the template database and applies every versioned
// migration to it, in numeric order.
func buildTemplate(ctx context.Context) error {
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return err
	}
	defer func() { _ = admin.Close() }()

	if _, err := admin.ExecContext(ctx, `CREATE DATABASE `+templateDB); err != nil {
		return fmt.Errorf("create template db: %w", err)
	}

	tmpl, err := sql.Open("pgx", dsnFor(templateDB))
	if err != nil {
		return err
	}
	defer func() { _ = tmpl.Close() }()

	if err := ApplyVersionedMigrations(ctx, tmpl); err != nil {
		return fmt.Errorf("apply migrations to template: %w", err)
	}

	// Marking it a template lets any connection clone it; without this,
	// CREATE DATABASE ... TEMPLATE requires the source to have no other
	// sessions, which races when tests run in parallel.
	if _, err := admin.ExecContext(ctx,
		`UPDATE pg_database SET datistemplate = true WHERE datname = $1`, templateDB); err != nil {
		return fmt.Errorf("mark template: %w", err)
	}
	return nil
}

// MigrationsDir locates db/migrations by walking up from the caller's working
// directory, so tests work from any package depth without a hardcoded path.
func MigrationsDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "db", "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not locate db/migrations from working directory")
}

// VersionedMigrationFiles returns the V*.sql files in ascending numeric order.
// Numeric, not lexicographic: V10 must come after V9, not after V1.
func VersionedMigrationFiles() ([]string, error) {
	dir, err := MigrationsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	type versioned struct {
		version int
		path    string
	}
	var files []versioned
	for _, e := range entries {
		m := versionedMigration.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("bad migration version in %q: %w", e.Name(), err)
		}
		files = append(files, versioned{v, filepath.Join(dir, e.Name())})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })

	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.path
	}
	return paths, nil
}

// RepeatableMigrationFiles returns the R__*.sql files from db/migrations and
// db/seeds, in name order — matching Flyway's ordering for repeatables.
func RepeatableMigrationFiles() ([]string, error) {
	dir, err := MigrationsDir()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, sub := range []string{dir, filepath.Join(filepath.Dir(dir), "seeds")} {
		entries, err := os.ReadDir(sub)
		if err != nil {
			continue // db/seeds is optional
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "R__") && strings.HasSuffix(e.Name(), ".sql") {
				paths = append(paths, filepath.Join(sub, e.Name()))
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// ApplyVersionedMigrations runs every V*.sql against db, in order.
func ApplyVersionedMigrations(ctx context.Context, db *sql.DB) error {
	paths, err := VersionedMigrationFiles()
	if err != nil {
		return err
	}
	return applyAll(ctx, db, paths)
}

// ApplyRepeatableMigrations runs every R__*.sql against db, in name order.
func ApplyRepeatableMigrations(ctx context.Context, db *sql.DB) error {
	paths, err := RepeatableMigrationFiles()
	if err != nil {
		return err
	}
	return applyAll(ctx, db, paths)
}

func applyAll(ctx context.Context, db *sql.DB, paths []string) error {
	for _, p := range paths {
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(body)) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(p), err)
		}
	}
	return nil
}

func dsnFor(dbName string) string {
	// adminDSN ends in /postgres?sslmode=disable; swap the database segment.
	return strings.Replace(adminDSN, "/postgres?", "/"+dbName+"?", 1)
}

// newClonedDatabase creates a fresh database from the migrated template and
// returns its name. Cloning is a PostgreSQL-side file copy, so this is cheap
// enough to do per test.
func newClonedDatabase(t *testing.T) string {
	t.Helper()

	cloneMu.Lock()
	cloneCount++
	name := fmt.Sprintf("test_%d_%d", os.Getpid(), cloneCount)
	cloneMu.Unlock()

	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	defer func() { _ = admin.Close() }()

	if _, err := admin.Exec(fmt.Sprintf(`CREATE DATABASE %s TEMPLATE %s`, name, templateDB)); err != nil {
		t.Fatalf("clone template database: %v", err)
	}

	t.Cleanup(func() {
		cleanup, err := sql.Open("pgx", adminDSN)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		// Terminate stragglers first; DROP DATABASE fails while any session
		// is still attached.
		_, _ = cleanup.Exec(
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1`, name)
		_, _ = cleanup.Exec(`DROP DATABASE IF EXISTS ` + name)
	})

	return name
}

// newEmptyDatabase creates a database with nothing in it — no migrations, no
// schema — for tests that exercise the migration set itself.
func newEmptyDatabase(t *testing.T) string {
	t.Helper()

	cloneMu.Lock()
	cloneCount++
	name := fmt.Sprintf("empty_%d_%d", os.Getpid(), cloneCount)
	cloneMu.Unlock()

	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	defer func() { _ = admin.Close() }()

	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("create empty database: %v", err)
	}

	t.Cleanup(func() {
		cleanup, err := sql.Open("pgx", adminDSN)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.Exec(
			`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1`, name)
		_, _ = cleanup.Exec(`DROP DATABASE IF EXISTS ` + name)
	})

	return name
}

// Terminate stops the shared container. Call from TestMain after m.Run().
func Terminate(ctx context.Context) {
	if container != nil {
		_ = container.Terminate(ctx)
	}
}

// ApplyFile runs a single .sql file against db — used by migration tests that
// need to re-apply one migration in isolation.
func ApplyFile(ctx context.Context, db *sql.DB, path string) error {
	return applyAll(ctx, db, []string{path})
}
