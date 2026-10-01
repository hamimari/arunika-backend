package contract_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"arunika_backend/tests/fixtures"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	if err := fixtures.StartPostgres(ctx); err != nil {
		if os.Getenv("CI") != "" {
			fmt.Fprintf(os.Stderr, "could not start PostgreSQL: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "skipping contract tests — %v\n", err)
		os.Exit(0)
	}
	code := m.Run()
	fixtures.Terminate(ctx)
	os.Exit(code)
}
