//go:build e2e

// Package e2e drives the real docker-compose stack — Postgres, Redis,
// Flyway, the backend, and the backoffice — for the cross-system business
// flows in design J of the automation-testing-strategy proposal.
//
// This is deliberately not Testcontainers: those tests (tests/db, tests/api)
// prove the Go code against a real database in-process, which is right for
// them. This package proves that the backend and backoffice, built and run
// exactly as they would be in production, actually work together — which
// means running the real Dockerfiles, not the Go code directly.
package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"arunika_backend/tests/fixtures"
)

const (
	composeProject = "arunika_e2e"
	appBaseURL     = "http://localhost:8090"
	adminBaseURL   = appBaseURL // same server; /admin is a route prefix, not a separate host
	// Matches docker-compose.test.yml's postgres.environment and flyway.command
	// exactly — both are pinned there rather than left to the base file's
	// ${DB_USER:-arunika} fallback, which would otherwise silently pick up
	// whatever real .env a developer has sitting in this directory.
	postgresDSN = "postgres://arunika_e2e:arunika_e2e_pw@localhost:5442/arunika_e2e?sslmode=disable"

	// Credentials seeded by db/seeds/R__seed_admin_user.sql (see the fix
	// note there — this only works because that hash now genuinely matches).
	AdminEmail    = "admin@arunika.id"
	AdminPassword = "admin123"

	// The three named users from db/seeds/test/seed.sql — see that file's own
	// header for what each one represents.
	FreeUserID        = "e2e00000-0000-0000-0000-000000000001"
	EntitledUserID    = "e2e00000-0000-0000-0000-000000000002"
	SubscriberUserID  = "e2e00000-0000-0000-0000-000000000003"
	FreeUserEmail     = "e2e-free@arunika.test"
	EntitledUserEmail = "e2e-entitled@arunika.test"
	SubscriberEmail   = "e2e-subscriber@arunika.test"
	SeedUserPassword  = "e2e-test-password"

	FreeArCard1ID  = "e2e0ac00-0000-0000-0000-000000000001"
	FreeArCard2ID  = "e2e0ac00-0000-0000-0000-000000000002"
	FreeArCard3ID  = "e2e0ac00-0000-0000-0000-000000000003"
	PaidArCard1ID  = "e2e0ac00-0000-0000-0000-000000000004"
	PaidArCard2ID  = "e2e0ac00-0000-0000-0000-000000000005"
	PaidProduct1ID = "e2e0ac00-0000-0000-0000-0000000000a4" // product behind PaidArCard1ID
	PaidProduct2ID = "e2e0ac00-0000-0000-0000-0000000000a5" // product behind PaidArCard2ID

	FreeDongeng1ID = "e2e0d000-0000-0000-0000-000000000001"
	FreeDongeng2ID = "e2e0d000-0000-0000-0000-000000000002"
	PaidDongeng1ID = "e2e0d000-0000-0000-0000-000000000003"
	PaidDongeng2ID = "e2e0d000-0000-0000-0000-000000000004"

	ContentBundlePackageID = "e2e0b000-0000-0000-0000-000000000001"
	SubscriptionPackageID  = "e2e0b000-0000-0000-0000-000000000002"
	BundlePlaySKU          = "e2e_sku_bundle"
	SubscriptionPlaySKU    = "e2e_sku_subscription"
	PaidArCard1PlaySKU     = "e2e_sku_ar_card_1"
	PaidArCard2PlaySKU     = "e2e_sku_ar_card_2"
)

// Stack is a running compose environment plus everything a flow test needs
// to talk to it and to the fake Android Publisher backing it.
type Stack struct {
	t        *testing.T
	BaseURL  string
	FakePlay *fixtures.FakePlay
	http     *http.Client
}

// Up brings the whole stack up fresh (any previous run's containers and
// volumes are removed first, so every test starts from an empty database),
// applies the deterministic seed corpus, and registers a cleanup that tears
// it down and — on failure — captures every service's logs plus a database
// dump before doing so.
//
// Skips (rather than fails) when Docker is unavailable, matching tests/db and
// tests/api; CI always has Docker, so this can never silently lose coverage
// there.
func Up(t *testing.T) *Stack {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		skipOrFail(t, "docker is not installed")
	}
	root := repoRoot(t)

	play, err := fixtures.NewFakePlayForDocker("host.docker.internal")
	require.NoError(t, err, "start fake Android Publisher")
	t.Cleanup(play.Close)

	writeEnvFile(t, root, play)

	compose := func(args ...string) *exec.Cmd {
		full := append([]string{"compose", "-p", composeProject,
			"-f", "docker-compose.yml", "-f", "docker-compose.test.yml"}, args...)
		cmd := exec.Command("docker", full...)
		cmd.Dir = root
		return cmd
	}

	// Always start from nothing: a prior run's volumes must never leak into
	// this one, and a stale container name from a crashed prior run must
	// never block `up`.
	runOrSkip(t, compose("down", "--volumes", "--remove-orphans"))

	up := compose("up", "-d", "--build", "--wait", "--wait-timeout", "180")
	var stderr bytes.Buffer
	up.Stderr = &stderr
	if err := up.Run(); err != nil {
		captureLogs(t, root)
		skipOrFailf(t, "docker compose up failed (docker may be unavailable in this environment): %v\n%s", err, stderr.String())
	}

	t.Cleanup(func() {
		if t.Failed() {
			captureLogs(t, root)
		}
		_ = compose("down", "--volumes", "--remove-orphans").Run()
	})

	applySeed(t, root)

	return &Stack{t: t, BaseURL: appBaseURL, FakePlay: play, http: &http.Client{Timeout: 10 * time.Second}}
}

func writeEnvFile(t *testing.T, root string, play *fixtures.FakePlay) {
	t.Helper()
	path := filepath.Join(root, "db", "seeds", "test", ".env.e2e.generated")

	// A complete set on its own — docker-compose.test.yml's env_file entry
	// replaces the base file's, it does not add to it.
	lines := []string{
		"DB_USER=arunika_e2e",
		"DB_PASSWORD=arunika_e2e_pw",
		"DB_NAME=arunika_e2e",
		"DB_SSLMODE=disable",
		"JWT_SECRET=e2e-test-secret-key-at-least-32-characters!!",
		"SMTP_HOST=localhost",
		"SMTP_PORT=1025",
		"SMTP_USER=e2e",
		"SMTP_PASS=e2e",
		"SMTP_EMAIL=noreply@e2e.test",
		"APP_DOMAIN=http://localhost:8090",
		"MIDTRANS_SERVER_KEY=e2e-unused",
		"MIDTRANS_CLIENT_KEY=e2e-unused",
		// A syntactically valid but unusable Firebase key — no flow here
		// exercises push notifications, but validateEnv() only checks
		// presence, and NotificationService fails closed if it's ever used.
		`FIREBASE_SERVICE_ACCOUNT_JSON={"type":"service_account"}`,
		"GOOGLE_PLAY_SERVICE_ACCOUNT_JSON=" + play.ServiceAccountJSON,
		"ANDROID_PUBLISHER_BASE_URL=" + play.AdvertisedURL,
		"ANDROID_PACKAGE_NAME=com.arunika",
	}
	require.NoError(t, os.WriteFile(path, []byte(joinLines(lines)), 0o600))
}

func applySeed(t *testing.T, root string) {
	t.Helper()
	db, err := sql.Open("pgx", postgresDSN)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	body, err := os.ReadFile(filepath.Join(root, "db", "seeds", "test", "seed.sql"))
	require.NoError(t, err)

	// The app container's own healthcheck only proves the HTTP server is up,
	// not that Flyway's migrate has committed — up --wait can return just
	// ahead of that on a slow first build. Retry briefly rather than
	// flaking on a table that is a few hundred milliseconds from existing.
	deadline := time.Now().Add(30 * time.Second)
	for {
		_, err = db.ExecContext(context.Background(), string(body))
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("apply test seed: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// captureLogs writes every service's logs and a database dump to
// E2E_ARTIFACT_DIR (default tests/e2e/artifacts, gitignored) so a failed run
// leaves evidence behind — the CI workflow uploads this directory.
func captureLogs(t *testing.T, root string) {
	t.Helper()
	dir := os.Getenv("E2E_ARTIFACT_DIR")
	if dir == "" {
		dir = filepath.Join(root, "tests", "e2e", "artifacts")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("could not create artifact dir %s: %v", dir, err)
		return
	}
	name := sanitizeName(t.Name())

	logs, _ := exec.Command("docker", "compose", "-p", composeProject,
		"-f", filepath.Join(root, "docker-compose.yml"),
		"-f", filepath.Join(root, "docker-compose.test.yml"),
		"logs", "--no-color").CombinedOutput()
	logPath := filepath.Join(dir, name+".compose.log")
	_ = os.WriteFile(logPath, logs, 0o644)

	dump, _ := exec.Command("docker", "exec", "arunika_e2e_postgres",
		"pg_dump", "-U", "arunika_e2e", "arunika_e2e").CombinedOutput()
	dumpPath := filepath.Join(dir, name+".db.sql")
	_ = os.WriteFile(dumpPath, dump, 0o644)

	t.Logf("compose logs: %s\ndatabase dump: %s", logPath, dumpPath)
}

func sanitizeName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == ' ' || r == os.PathSeparator {
			return '_'
		}
		return r
	}, s)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate repository root (docker-compose.yml not found)")
	return ""
}

func joinLines(lines []string) string {
	var b bytes.Buffer
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

// skipOrFail/skipOrFailf treat an unusable Docker environment as a skip, not
// a failure — matching tests/db and tests/api. A CI runner with a broken
// Docker daemon is a CI problem this suite should surface loudly, so it
// fails there instead (guarded by the same CI env var those packages use).
func skipOrFail(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Fatal(reason)
	}
	t.Skip(reason)
}

func skipOrFailf(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	skipOrFail(t, fmt.Sprintf(format, args...))
}

func runOrSkip(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		skipOrFailf(t, "docker compose down failed: %v\n%s", err, stderr.String())
	}
}
