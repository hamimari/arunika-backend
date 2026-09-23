package db_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"arunika_backend/tests/fixtures"
)

// TestMain starts one PostgreSQL container for this package and builds the
// migrated template database that every test clones from.
//
// Skipping rather than failing when Docker is unavailable keeps `go test ./...`
// usable on a machine without it; CI always has Docker, so coverage is never
// silently lost there.
func TestMain(m *testing.M) {
	ctx := context.Background()

	if err := fixtures.StartPostgres(ctx); err != nil {
		if os.Getenv("CI") != "" {
			fmt.Fprintf(os.Stderr, "could not start PostgreSQL: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "skipping database tests — could not start PostgreSQL: %v\n", err)
		os.Exit(0)
	}

	code := m.Run()
	fixtures.Terminate(ctx)
	os.Exit(code)
}
