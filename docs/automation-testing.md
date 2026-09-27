# Automation testing — arunika-backend

How the API is tested and how to run each suite again. Other repos:
[app](../../arunika_app/docs/automation-testing.md),
[backoffice](../../arunika-backoffice/docs/automation-testing.md). Rationale and
history: `arunika_app/openspec/changes/add-automation-testing-strategy/`.

## Suites

| Suite | Where | Needs | CI |
|---|---|---|---|
| Unit (sqlmock, miniredis) | `handlers/ services/ middlewares/ routes/ utils/` | nothing | every PR |
| DB — real Postgres, real constraints | `tests/db` | Docker | every PR |
| API — full router + middleware + Postgres | `tests/api` | Docker | every PR |
| Contract — routes vs `openapi.yaml` | `tests/contract` | Docker | every PR |
| **Security regression** | `tests/security` | Docker | every PR |
| Cross-system E2E — real docker-compose stack | `tests/e2e` (tag `e2e`) | Docker | on merge |
| **Smoke** — a deployed environment, read-only | `tests/smoke` (tag `smoke`) | a URL | manual |
| Flaky detection | `scripts/flaky_detect.py` | Docker | nightly |

`go test ./...` runs everything except the tagged suites. With `CI=true` a suite
that cannot start Postgres **fails** instead of skipping, so coverage cannot be
lost silently. No suite needs a Google credential: `fixtures.FakePlay` stands in
for the Play Developer API.

## Everyday loop

```bash
make test-fast     # go test ./...
make test-all      # lint + build + -race + coverage ratchet (what a PR runs)
```

`test-all` needs `gotestsum` v1.13.0 and `golangci-lint` v2.13.2 (see the README).
After an intentional coverage change: `make coverage-baseline`.

Helpers live in `tests/fixtures/`: builders (`NewUser`, `NewProduct`,
`NewOrderForProduct`, …), `FreshDB` (a cloned, migrated database per test),
`FakePlay`.

## Security suite — `tests/security`

```bash
go test ./tests/security/ -count=1 -v
go test ./tests/security/ -count=1 -run EveryAdminRoute -v      # one test
```

34 tests (59 with subtests). It assembles the real router like `tests/api`, but
asserts policy:

- **Credentials** — missing, malformed, expired, wrong-key, wrong-algorithm,
  `alg:none`, no-`exp` and no-`jti` tokens are all 401; a logged-out access token
  is dead (Redis blacklist); expired, logout-revoked, replayed and deleted-account
  refresh tokens are refused.
- **Horizontal escalation** — user A cannot read or change user B's orders,
  notifications, fairy-tale history, profile or child's growth records; a foreign
  resource must look exactly like a missing one.
- **Vertical escalation** — every `/admin/*` route (93 today), discovered by
  reading `Router.Routes()`, returns 403 to a user token. A new admin route is
  covered automatically; a floor on the route count stops discovery breaking
  silently.
- **Purchase tampering** — product-id swap, a token Google rejects for another
  package, fabricated token, one token replayed against a second order or another
  account, verifying someone else's order, cancelled/pending purchases.
- **Injection and leakage** — six SQL payloads across 16 endpoints × 13 parameter
  names, then every route hit with hostile ids and bodies: no 500s and no SQL,
  driver, stack-trace or path fragments in any response. Login and
  forgot-password must not reveal which accounts exist.

**Adding to it.** A new route needs no admin-escalation or leakage test. Add a
horizontal test in `authz_test.go` if it returns per-user data. If you find a
defect you can't fix immediately, write the test for the *correct* behaviour and
`t.Skip("KNOWN VULNERABILITY: … <location>")` — never weaken the assertion — and
delete the skip in the fix. Currently no such skips exist.

Harness note: `TestMain` `chdir`s to the repo root because handlers load
`templates/` by relative path; without it template-backed routes return 500.

## Cross-system E2E — `tests/e2e`

Starts the real compose stack (Postgres, Redis, Flyway, backend, backoffice) via
`docker-compose.test.yml` and drives it over HTTP. The backoffice is built from
`../arunika-backoffice`, so that repo must sit next to this one.

```bash
make test-e2e      # run the flows, tear down
make e2e-hold      # keep the stack up (api :8090, backoffice :3010) for the
                   # backoffice Playwright specs and the app's emulator flows; Ctrl-C stops it
```

`e2e-hold` prints `E2E STACK READY` when it is up. On a failing test the suite
saves `docker compose logs` and a `pg_dump` to `E2E_ARTIFACT_DIR`. Give Docker
enough memory (6 GB+) and don't run it alongside an emulator and an IDE on a small
machine.

## Smoke — `tests/smoke`

Read-only checks for a deployed environment: `/health`, five anonymous content
lists, anonymous rejection on `/orders` and `/admin/users`, and a canary login +
`GET /orders` (which logs out afterwards). Nothing is seeded or purchased.

```bash
SMOKE_BASE_URL=https://api.example.com \
SMOKE_CANARY_EMAIL=canary@example.com SMOKE_CANARY_PASSWORD=... \
go test -tags smoke ./tests/smoke/ -count=1 -v
```

Without the canary variables only the anonymous checks run. **It has not been run
against a real deployment** — none is configured yet.

## Flaky detection

```bash
mkdir -p reports
for i in 1 2 3; do gotestsum --junitfile reports/nightly-$i.xml -- -race -count=1 ./... ; done
python3 scripts/flaky_detect.py reports/nightly-*.xml
```

Exit 1 lists tests that passed *and* failed on the same commit (flaky) separately
from tests that failed every run (broken). Quarantine policy: README "Flaky tests".

## CI

| File | Trigger | What |
|---|---|---|
| `pr.yml` | pull request | lint, `go test -race ./...`, coverage ratchet |
| `merge.yml` | push, `app-merged` / `backoffice-merged` dispatch | cross-system E2E |
| `nightly.yml` | 02:00 WIB, manual | 3× full suite + flaky comparison |

Neither `merge.yml` nor `nightly.yml` has run on GitHub yet — run each once with
*Run workflow*. Cross-repo checkouts use `CROSS_REPO_TOKEN`.

## Known gaps

- `refresh_tokens.expires_at` is a zoneless `TIMESTAMP`: expiry is only correct
  when the server runs in UTC (a test in WIB exposed it).
- No deploy pipeline, so no `release.yml`; the smoke suite is ready for one.
