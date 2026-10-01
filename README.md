# Arunika Backend

REST API backend for the Arunika application, built with Go (Gin), PostgreSQL, and Redis.

## Tech Stack

- **Language**: Go 1.25
- **Framework**: [Gin](https://github.com/gin-gonic/gin)
- **Database**: PostgreSQL 15 (via GORM)
- **Cache / Session**: Redis 7
- **Migrations**: Flyway
- **Auth**: JWT (golang-jwt/jwt v5)
- **Email**: SMTP (gomail)
- **Push Notifications**: Firebase Cloud Messaging
- **Payments**: Midtrans Snap
- **Containerisation**: Docker / Docker Compose

## Project Structure

```
.
├── config/             # Database and Redis initialisation
├── db/
│   ├── migrations/     # Flyway SQL migration files (V1__ … V20__)
│   └── seeds/          # Repeatable seed scripts (R__)
├── handlers/           # HTTP request handlers
├── middlewares/        # Gin middleware (JWT, admin auth, CORS, security headers)
├── models/             # GORM models / domain structs
├── registry/           # Service registry (dependency wiring)
├── routes/             # Router setup
├── services/           # Business logic
├── templates/          # Email HTML templates
├── utils/              # Shared utilities (JWT helpers, etc.)
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── main.go
```

---

## Getting Started (Local Development)

### Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Docker & Docker Compose](https://docs.docker.com/get-docker/)

### 1. Clone the repository

```bash
git clone <repo-url>
cd "arunika backend"
```

### 2. Configure environment variables

```bash
cp .env.example .env
```

Edit `.env` and fill in all required values. See the [Environment Variables](#environment-variables) section below for the full reference.

### 3. Start the full stack with Docker Compose

This is the recommended approach — it starts PostgreSQL, Redis, runs Flyway migrations, then starts the API server, all in the correct order.

```bash
docker compose up -d
```

The API will be available at `http://localhost:8080`.

Useful commands:

```bash
# Follow API logs
docker compose logs -f app

# Restart only the app after a code change
docker compose up -d --build app

# Stop all containers (data volumes are preserved)
docker compose down

# Stop and remove all data volumes (full reset)
docker compose down -v
```

### 4. Run locally without Docker (app only)

Use this if you prefer to run the Go server directly while still using Docker for PostgreSQL and Redis.

```bash
# Start only the infrastructure services
docker compose up -d postgres redis

# Wait for migrations to finish
docker compose run --rm flyway

# Run the server
go run main.go
```

The server starts on `http://localhost:8080` by default.

### 5. Build the binary manually

```bash
go build -o arunika_backend .
./arunika_backend
```

---

## Running Tests

```bash
make test-fast    # inner loop: go test ./...
make test-all     # everything CI runs on a pull request
```

`make test-all` needs two tools once per machine:

```bash
# v1.12.0 and earlier fail to build on Go 1.26
go install gotest.tools/gotestsum@v1.13.0
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
```

465 tests (including subtests) cover services, handlers, middlewares, routes
and JWT utilities using `sqlmock` and `miniredis`. CI runs them under `-race`
on every pull request (`.github/workflows/pr.yml`).

### Security, smoke and E2E suites

`go test ./...` already includes `tests/security` (token forgery, escalation,
purchase tampering, injection, error leakage). Other suites are opt-in:

```bash
go test ./tests/security/ -count=1 -v                    # security only
make test-e2e                                            # docker-compose E2E
SMOKE_BASE_URL=... go test -tags smoke ./tests/smoke/    # deployed environment
python3 scripts/flaky_detect.py reports/nightly-*.xml    # compare repeated runs
```

Full guide: [`docs/automation-testing.md`](docs/automation-testing.md).

### Coverage ratchet

`make test-all` compares per-package coverage against `coverage-baseline.json`
and reports any package that dropped. It is **report-only** today — it prints
regressions without failing the build. After intentional changes:

```bash
make coverage-baseline
```

Current baseline: `routes` 99.5%, `utils` 92.3%, `services` 62.4%,
`middlewares` 45.1%, `handlers` 28.8%, `models` 0%.

> `models` at 0% is the entitlement data layer (`GrantEntitlement`,
> `HasEntitlement`, `FindPackageItems`). It is the highest-priority gap and is
> addressed in Phase 2 of the automation testing strategy.

### Flaky tests

A flaky test is one whose result changes between runs on an unchanged commit.
The policy across all three Arunika repositories:

- **Never add a retry to hide one.** Retries are permitted only in the E2E
  tier, where a real device or browser has genuine nondeterminism. A
  non-deterministic unit or integration test is a real defect in the test or
  the code.
- **Quarantine within one working day.** Mark it `t.Skip("flaky — see #NNN")`
  so it stops blocking merges while still being visible.
- **Assign an owner and a two-week expiry.** At expiry it is fixed or
  deleted. A permanently quarantined test is worse than no test — it burns CI
  time and erodes trust in the suite.
- **Common causes here:** time-dependent assertions (inject a clock rather
  than comparing against `time.Now()`), shared Redis or database state between
  tests, and goroutine ordering. Run `go test -race` before assuming a
  flake is environmental.

---

## Deployment

### Option 1 — Docker Compose on a single server (VPS / EC2 / etc.)

This is suitable for staging environments or small-scale production.

**1. Copy required files to the server**

```bash
scp Dockerfile docker-compose.yml .env.example user@your-server:/opt/arunika/
```

Or clone the repository directly on the server.

**2. Create the production `.env`**

```bash
cp .env.example .env
# Edit .env with production values — use strong secrets
nano .env
```

**3. Build and start**

```bash
docker compose up -d --build
```

**4. Check status**

```bash
docker compose ps
docker compose logs -f app
```

**5. Update to a new version**

```bash
git pull
docker compose up -d --build app
```

Flyway will automatically run any new migration files on the next start of the `flyway` service. To run migrations independently before restarting the app:

```bash
docker compose run --rm flyway
```

---

### Option 2 — Build image in CI and push to a registry

Use this for a proper CI/CD pipeline (GitHub Actions, GitLab CI, etc.).

**Build and push**

```bash
docker build -t your-registry/arunika-backend:${GIT_SHA} .
docker push your-registry/arunika-backend:${GIT_SHA}
```

**Deploy on the server**

```bash
# Pull the new image
docker pull your-registry/arunika-backend:${GIT_SHA}

# Update docker-compose.yml to use the pinned image tag instead of `build:`
# then:
docker compose up -d
```

---

## Environment Variables

All variables are required unless marked optional.

| Variable | Description | Example |
|---|---|---|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL user | `arunika` |
| `DB_PASSWORD` | PostgreSQL password | `changeme` |
| `DB_NAME` | Database name | `arunika` |
| `DB_SSLMODE` | SSL mode (`disable` / `require`) | `disable` |
| `REDIS_HOST` | Redis host | `localhost` |
| `REDIS_PORT` | Redis port (optional) | `6379` |
| `REDIS_PASSWORD` | Redis password (optional) | _(empty)_ |
| `JWT_SECRET` | JWT signing secret — **min 32 chars** | `change-me-32-char-min` |
| `SMTP_HOST` | SMTP server host | `smtp.example.com` |
| `SMTP_PORT` | SMTP server port | `587` |
| `SMTP_USER` | SMTP sender address | `no-reply@example.com` |
| `SMTP_PASS` | SMTP password | `changeme` |
| `SMTP_EMAIL` | From address on outgoing mail (optional, defaults to `SMTP_USER`) | `no-reply@example.com` |
| `APP_DOMAIN` | Public URL of the application | `https://app.example.com` |
| `PORT` | HTTP port the server listens on (optional) | `8080` |
| `MIDTRANS_SERVER_KEY` | Midtrans server key | `SB-Mid-server-…` |
| `MIDTRANS_CLIENT_KEY` | Midtrans client key | `SB-Mid-client-…` |
| `FIREBASE_SERVICE_ACCOUNT_JSON` | Firebase service account JSON (raw content or file path) | `{"type":"service_account",…}` |

> **Fail-fast**: the application will refuse to start if any required variable is missing.
> **Security**: never commit `.env` to version control.

---

## API Reference

### Health

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | No | Health check — returns `{"status":"ok"}` |

### Auth

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/auth/login` | No | Login with email & password |
| POST | `/auth/signup` | No | Register a new account |
| POST | `/auth/send-otp` | No | Send OTP to email |
| POST | `/auth/refresh-token` | JWT | Refresh access token |
| POST | `/auth/logout` | JWT | Logout / invalidate token |
| POST | `/forgot-password` | No | Request password reset email |
| POST | `/reset-password` | No | Reset password with token |

### User

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/user/:id` | JWT | Get user profile |
| PUT | `/user` | JWT | Update user profile |

### AR Cards

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/ar/cards` | No | List all AR cards |
| GET | `/ar/cards/:id` | JWT | Get AR card by ID |
| GET | `/ar/categories` | No | List AR card categories |

### Categories

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/categories` | JWT | List all content categories |

### Fairy Tales (Dongeng)

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/fairy-tales` | No | List fairy tales (paginated) |
| GET | `/fairy-tales/:id` | No | Get fairy tale detail |

### Tracing

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/tracing/items` | JWT | List tracing items |
| POST | `/tracing/progress` | JWT | Save tracing progress |

### Counting

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/counting/questions` | JWT | List counting questions |
| POST | `/counting/progress` | JWT + Premium | Save counting progress |

### Badges

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/badges` | JWT | Get user badges |

### Payment

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/payment/create` | JWT | Create Midtrans Snap transaction |
| POST | `/payment/webhook` | No | Midtrans webhook callback |

### Notifications

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/notifications` | JWT | List user notifications |
| PATCH | `/notifications/:id/read` | JWT | Mark notification as read |
| POST | `/notifications/token` | JWT | Register FCM device token |

### Growth Records

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/growth` | JWT | Save growth record |
| GET | `/growth` | JWT | Get growth history |
| PUT | `/growth/:id` | JWT | Update growth record |

### Printable PDF

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/ar/printable-pdf` | No | Generate printable AR card PDF |

### Admin — Auth

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/admin/auth/login` | No | Admin login |
| POST | `/admin/auth/refresh` | No | Refresh admin token |
| POST | `/admin/auth/logout` | Admin JWT | Admin logout |

### Admin — Analytics

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/admin/analytics/dau` | Admin JWT | Daily active users |
| GET | `/admin/analytics/new-users` | Admin JWT | New user registrations |
| GET | `/admin/analytics/popular-features` | Admin JWT | Feature usage stats |
| GET | `/admin/analytics/payments` | Admin JWT | Payment metrics |
| GET | `/admin/analytics/subscription-stats` | Admin JWT | Subscription statistics |

### Admin — Users

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/admin/users` | Admin JWT | List all users |
| GET | `/admin/users/:id` | Admin JWT | Get user detail |
| PATCH | `/admin/users/:id/permission` | Admin JWT | Grant / revoke premium |

### Admin — Payments

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/admin/payments` | Admin JWT | List payment transactions |
| GET | `/admin/payments/:id` | Admin JWT | Get transaction detail |

### Admin — Content (Banners, Fairy Tales, AR Cards, Tracing, Counting, Badges, Categories)

All content endpoints follow the same pattern under `/admin/content/`:

| Method | Path pattern | Auth | Description |
|---|---|---|---|
| GET | `/admin/content/{resource}` | Admin JWT | List items |
| POST | `/admin/content/{resource}` | Admin JWT | Create item |
| GET | `/admin/content/{resource}/:id` | Admin JWT | Get item |
| PUT | `/admin/content/{resource}/:id` | Admin JWT | Update item |
| DELETE | `/admin/content/{resource}/:id` | Admin JWT | Delete item |
| PATCH | `/admin/content/{resource}/:id/visibility` | Admin JWT | Toggle visibility |

Where `{resource}` is one of: `banners`, `fairy-tales`, `ar-cards`, `tracing-items`, `counting-questions`, `badges`, `categories`.

Banners also support:

| Method | Path | Auth | Description |
|---|---|---|---|
| PATCH | `/admin/content/banners/:id/active` | Admin JWT | Toggle active status |
