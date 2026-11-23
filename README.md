# E-Commerce API

A REST API for an e-commerce shop, written in Go. It uses PostgreSQL for data and S3 (LocalStack in development) for file uploads.

> Work in progress, built in stages. Stages 1 (database), 2 (authentication) and 3 (products and categories) are done. See the [learning path](docs/learn/README.md) for the stages and a lesson on each.

## Design highlights

- **Money is stored as integer cents** (`price_cents BIGINT`), never floats, so totals are always exact.
- **The database enforces the business rules**: stock can't go negative, emails are unique, order statuses are a fixed set.
- **Order items snapshot the name and price at checkout**, so later product edits never change past orders.
- **The schema is managed by versioned SQL migrations**, not generated from structs at runtime.
- **Short-lived access tokens with rotating refresh tokens.** Reusing an old refresh token is treated as theft and ends all of that user's sessions.
- **Passwords are hashed with bcrypt and refresh tokens with SHA-256**, so a leaked database doesn't leak logins.
- **Errors never leak internals.** Clients get a stable error code and a safe message; the real error is logged with a request ID.
- **Admin routes are protected as a group** (401 if not logged in, 403 if not an admin), and no endpoint can grant the admin role.
- **Paginated, filterable product lists** with a capped page size, stable sort order, whitelisted sort options and parameterized queries (no SQL injection).
- **Partial updates that handle zero correctly**: `PATCH {"stock": 0}` really sets stock to 0.
- **Row locks prevent races**, e.g. deleting a category while someone adds a product to it.

## Tech stack

- **Go** 1.26+
- **PostgreSQL** 17 (via Docker)
- **LocalStack** 4.14 for a local S3 bucket (via Docker)
- **golangci-lint** v2 for linting
- **golang-migrate** for database migrations

## Prerequisites

- [Go](https://go.dev/dl/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- GNU Make (Windows: `winget install ezwinports.make`)
- [golangci-lint](https://golangci-lint.run/welcome/install/)
- [golang-migrate](https://github.com/golang-migrate/migrate) (for migrations)

## Getting started

```bash
git clone https://github.com/UncleKweks/E-Commerce.git
cd E-Commerce

# Create your local config, then set JWT_SECRET in it
# (generate one with: openssl rand -hex 32)
cp .env.example .env

# Start Postgres and LocalStack
make docker-up

# Create the database tables
make migrate-up

# Optional: load sample categories and products
make seed

# Build and run the API on http://localhost:8080
make run
```

Press Ctrl+C to stop it. It finishes in-flight requests before exiting.

## API

All endpoints are under `/api/v1`. Successful responses look like `{"data": ...}`, and errors look like `{"error": {"code": "...", "message": "...", "details": {...}}}`.

| Method | Path | Auth | What it does |
| --- | --- | --- | --- |
| `GET` | `/health` | | Reports whether the API and database are up |
| `POST` | `/api/v1/auth/register` | | Create an account; returns tokens |
| `POST` | `/api/v1/auth/login` | | Log in; returns tokens |
| `POST` | `/api/v1/auth/refresh` | | Swap a refresh token for a new pair |
| `POST` | `/api/v1/auth/logout` | | Revoke a refresh token |
| `GET` | `/api/v1/users/me` | Bearer token | The logged-in user's profile |
| `GET` | `/api/v1/categories` | | List active categories |
| `GET` | `/api/v1/products` | | List active products (paginated, see filters below) |
| `GET` | `/api/v1/products/{id}` | | One active product |
| `GET` | `/api/v1/admin/categories` | Admin | List all categories, including hidden ones |
| `POST` | `/api/v1/admin/categories` | Admin | Create a category |
| `PATCH` | `/api/v1/admin/categories/{id}` | Admin | Change some fields of a category |
| `DELETE` | `/api/v1/admin/categories/{id}` | Admin | Delete a category (only if it has no products) |
| `GET` | `/api/v1/admin/products` | Admin | List all products, including hidden ones |
| `GET` | `/api/v1/admin/products/{id}` | Admin | One product, even if hidden |
| `POST` | `/api/v1/admin/products` | Admin | Create a product |
| `PATCH` | `/api/v1/admin/products/{id}` | Admin | Change some fields of a product |
| `DELETE` | `/api/v1/admin/products/{id}` | Admin | Delete a product |

Login and register are rate-limited per IP address. Access tokens last 15 minutes; refresh tokens last 7 days and work only once.

**Product list filters** (all optional): `page` (default 1), `page_size` (default 20, max 100), `category_id`, `q` (search in the name), `min_price` and `max_price` (in cents), `in_stock=true`, and `sort` (`newest`, `price_asc`, `price_desc` or `name`). List replies include `meta` with `page`, `page_size`, `total` and `total_pages`.

**Becoming an admin:** register normally, then run `make admin email=you@example.com` and log in again. There is deliberately no API endpoint for this.

Try it with curl:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"ada@example.com","password":"supersecret1","first_name":"Ada","last_name":"Obi"}'

# Use the access_token from the response:
curl http://localhost:8080/api/v1/users/me -H "Authorization: Bearer <access_token>"
```

## Configuration

All settings live in `.env` (never committed). `.env.example` lists every variable:

| Variable | Purpose |
| --- | --- |
| `PORT` | Port the API listens on |
| `GIN_MODE` | `debug`, `release` or `test` |
| `CORS_ALLOWED_ORIGINS` | Comma-separated frontend URLs allowed to call the API from a browser |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | PostgreSQL connection. Docker Compose and the Makefile read these too |
| `JWT_SECRET` | Required, at least 32 characters. Signs access tokens; anyone who has it can forge logins |
| `JWT_EXPIRATION`, `REFRESH_TOKEN_EXPIRATION` | Token lifetimes as Go durations, e.g. `15m`, `168h` |
| `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_S3_BUCKET`, `AWS_S3_ENDPOINT` | S3 storage (points at LocalStack locally) |
| `UPLOAD_PATH`, `MAX_UPLOAD_SIZE` | File upload location and size limit in bytes |

Postgres only reads `DB_PASSWORD` when its data volume is first created. If you change it later, reset the volume:

```bash
docker compose --env-file .env -f docker/docker-compose.yml down -v
make docker-up
```

## Troubleshooting

**`password authentication failed for user "postgres"`**: another PostgreSQL (often one installed in WSL or directly on Windows) is already using port 5432, so the app connects to the wrong database. Set `DB_PORT=5433` in `.env` and run `make docker-up` again. Docker Compose and the Makefile both read that value.

**`MAX_UPLOAD_SIZE must be a whole number`** (or another setting that looks correct in `.env`): something has loaded `.env` into your terminal with the wrong parser, and terminal values take priority over the file. In VS Code, the Python extension's `python.terminal.useEnvFile` setting does this. Turn it off and open a new terminal. Keep comments in `.env` on their own lines, never after a value.

## Make commands

| Command | What it does |
| --- | --- |
| `make help` | List the commands |
| `make build` | Compile to `bin/api.exe` |
| `make run` | Build, then run the API |
| `make dev` | Run with `go run` (no build step) |
| `make lint` | Run golangci-lint on all packages |
| `make test` | Run all tests |
| `make docker-up` | Start Postgres and LocalStack |
| `make docker-down` | Stop the containers |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back the most recent migration |
| `make migrate-version` | Show which migration the database is on |
| `make migrate-create name=NAME` | Create a new numbered pair of migration files |
| `make seed` | Load sample categories and products (safe to repeat) |
| `make admin email=EMAIL` | Make an existing user an admin |

## Project structure

```text
cmd/api/            Entry point: starts the HTTP server, handles shutdown
cmd/admin/          Command that turns a user into an admin
internal/auth/      Password hashing, access tokens (JWT), refresh tokens
internal/config/    Loads and validates configuration from the environment
internal/database/  PostgreSQL connection and pool (GORM)
internal/dto/       JSON request and response shapes
internal/handlers/  HTTP handlers: read requests, call services, write responses
internal/logger/    Application logging
internal/middleware/ Auth, request logging, rate limiting, CORS, body limits
internal/models/    Go structs that mirror the database tables
internal/response/  The standard JSON response and error format
internal/server/    Router: connects URLs to handlers and middleware
internal/services/  Business logic
db/migrations/      Numbered SQL migrations that define the schema
db/seed.sql         Sample data for local development
docker/             Docker Compose file for Postgres and LocalStack
docs/learn/         Stage-by-stage lessons explaining how it's built
.env.example        Template for local configuration
.golangci.yml       Linter configuration
Makefile            Build, run, lint, Docker and migration commands
```
