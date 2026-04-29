# Arunika Backend

REST API backend for the Arunika application, built with Go (Gin), PostgreSQL, and Redis.

## Tech Stack

- **Language**: Go 1.23
- **Framework**: [Gin](https://github.com/gin-gonic/gin)
- **Database**: PostgreSQL (via GORM)
- **Cache / Session**: Redis
- **Migrations**: Flyway
- **Auth**: JWT (golang-jwt/jwt v5)
- **Email**: SMTP (gomail)
- **Containerisation**: Docker / Docker Compose

## Prerequisites

- Go 1.23+
- PostgreSQL 15+
- Redis
- Docker & Docker Compose (optional, for local infra)

## Getting Started

### 1. Clone the repository

```bash
git clone <repo-url>
cd "arunika backend"
```

### 2. Configure environment variables

Copy the example env file and fill in the values:

```bash
cp .env.example .env
```

| Variable        | Description                              | Example                        |
|-----------------|------------------------------------------|--------------------------------|
| `DB_HOST`       | PostgreSQL host                          | `localhost`                    |
| `DB_PORT`       | PostgreSQL port                          | `5432`                         |
| `DB_USER`       | PostgreSQL user                          | `postgres`                     |
| `DB_PASSWORD`   | PostgreSQL password                      | `changeme`                     |
| `DB_NAME`       | Database name                            | `arunika`                      |
| `DB_SSLMODE`    | SSL mode                                 | `disable`                      |
| `REDIS_HOST`    | Redis host                               | `localhost`                    |
| `REDIS_PORT`    | Redis port                               | `6379`                         |
| `REDIS_PASSWORD`| Redis password (leave empty if none)     |                                |
| `JWT_SECRET`    | Secret key for JWT (min 32 chars)        | `change-me-to-a-long-secret`   |
| `SMTP_HOST`     | SMTP server host                         | `smtp.example.com`             |
| `SMTP_PORT`     | SMTP server port                         | `587`                          |
| `SMTP_USER`     | SMTP username / sender address           | `no-reply@example.com`         |
| `SMTP_PASS`     | SMTP password                            | `changeme`                     |
| `APP_DOMAIN`    | Public URL of the application            | `https://app.example.com`      |
| `PORT`          | HTTP port the server listens on          | `8080`                         |

### 3. Start infrastructure (PostgreSQL + Flyway migrations)

```bash
docker-compose up -d
```

This starts a PostgreSQL instance and runs all Flyway migrations automatically.

### 4. Run the server

```bash
go run main.go
```

The server starts on `http://localhost:8080` by default.

### 5. Build the binary

```bash
go build -o arunika_backend .
./arunika_backend
```

## API Endpoints

### Health

| Method | Path      | Auth | Description          |
|--------|-----------|------|----------------------|
| GET    | `/health` | No   | Health check         |

### Auth

| Method | Path                   | Auth     | Description                  |
|--------|------------------------|----------|------------------------------|
| POST   | `/auth/login`          | No       | Login with email & password  |
| POST   | `/auth/signup`         | No       | Register a new account       |
| POST   | `/auth/send-otp`       | No       | Send OTP to email            |
| POST   | `/auth/refresh-token`  | JWT      | Refresh access token         |
| POST   | `/auth/logout`         | JWT      | Logout / invalidate token    |
| POST   | `/forgot-password`     | No       | Request password reset email |
| POST   | `/reset-password`      | No       | Reset password with token    |

### User

| Method | Path          | Auth | Description           |
|--------|---------------|------|-----------------------|
| GET    | `/user/:id`   | JWT  | Get user by ID        |
| PUT    | `/user`       | JWT  | Update user profile   |

### AR Cards

| Method | Path              | Auth | Description        |
|--------|-------------------|------|--------------------|
| GET    | `/ar/cards/:id`   | JWT  | Get AR card by ID  |

### Categories

| Method | Path           | Auth | Description        |
|--------|----------------|------|--------------------|
| GET    | `/categories`  | JWT  | List all categories|

### Fairy Tales (Dongeng)

| Method | Path                 | Auth | Description              |
|--------|----------------------|------|--------------------------|
| GET    | `/fairy-tales`       | JWT  | List all fairy tales     |
| GET    | `/fairy-tales/:id`   | JWT  | Get fairy tale detail    |

## Project Structure

```
.
├── config/         # Database and Redis initialisation
├── db/
│   └── migrations/ # Flyway SQL migration files
├── handlers/       # HTTP request handlers (controllers)
├── middlewares/    # Gin middleware (JWT, CORS, security headers, error handling)
├── models/         # GORM models / domain structs
├── registry/       # Dependency injection / service registry
├── routes/         # Router setup
├── services/       # Business logic
├── templates/      # Email templates
├── utils/          # Shared utilities
├── docker-compose.yml
├── flyway.conf
├── go.mod
└── main.go
```

## Running Tests

```bash
go test ./...
```

## Environment Notes

- The application performs a **fail-fast check** on startup — it will refuse to start if any required environment variable is missing.
- `JWT_SECRET` must be **at least 32 characters** long.
- In production, tighten the CORS `AllowOrigins` in `routes/router.go` to your actual domain(s).
