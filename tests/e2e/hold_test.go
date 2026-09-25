//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

// TestStackHold brings the stack up and keeps it running until interrupted, so
// other suites (the backoffice Playwright specs) can drive it. Skipped unless
// E2E_HOLD=1, so a normal `go test -tags e2e` never blocks.
func TestStackHold(t *testing.T) {
	if os.Getenv("E2E_HOLD") != "1" {
		t.Skip("set E2E_HOLD=1 to hold the stack up for external suites")
	}
	Up(t)
	fmt.Println("E2E STACK READY: api http://localhost:8090  backoffice http://localhost:3010")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}
