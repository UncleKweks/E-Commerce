# E-Commerce API

A REST API for an e-commerce shop, written in Go. It uses PostgreSQL for data and S3 (LocalStack in development) for file uploads.

> Work in progress, built in stages. Stage 1 (database schema and migrations) is done; the API currently connects to the database and exits. See the [learning path](docs/learn/README.md) for the stages and a lesson on each.

## Design highlights

- **Money is stored as integer cents** (`price_cents BIGINT`), never floats, so totals are always exact.
- **The database enforces the business rules**: stock can't go negative, emails are unique, order statuses are a fixed set.
- **Order items snapshot the name and price at checkout**, so later product edits never change past orders.
- **The schema is managed by versioned SQL migrations**, not generated from structs at runtime.

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

# Create your local config and fill in your own values
cp .env.example .env

# Start Postgres and LocalStack
make docker-up

# Create the database tables
make migrate-up

# Build and run the API
make run
```

## Configuration

All settings live in `.env` (never committed). `.env.example` lists every variable:

| Variable | Purpose |
| --- | --- |
| `PORT` | Port the API listens on |
| `GIN_MODE` | `debug` or `release` |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | PostgreSQL connection. Docker Compose and the Makefile read these too |
| `JWT_SECRET`, `JWT_EXPIRATION`, `REFRESH_TOKEN_EXPIRATION` | Authentication tokens |
| `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_S3_BUCKET`, `AWS_S3_ENDPOINT` | S3 storage (points at LocalStack locally) |
| `UPLOAD_PATH`, `MAX_UPLOAD_SIZE` | File upload location and size limit in bytes |

Postgres only reads `DB_PASSWORD` when its data volume is first created. If you change it later, reset the volume:

```bash
docker compose --env-file .env -f docker/docker-compose.yml down -v
make docker-up
```

## Make commands

| Command | What it does |
| --- | --- |
| `make help` | List the commands |
| `make build` | Compile to `bin/api.exe` |
| `make run` | Build, then run the API |
| `make dev` | Run with `go run` (no build step) |
| `make lint` | Run golangci-lint on all packages |
| `make docker-up` | Start Postgres and LocalStack |
| `make docker-down` | Stop the containers |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back the most recent migration |
| `make migrate-version` | Show which migration the database is on |
| `make migrate-create name=NAME` | Create a new numbered pair of migration files |

## Project structure

```text
cmd/api/            Application entry point
internal/config/    Loads configuration from the environment
internal/database/  PostgreSQL connection and pool (GORM)
internal/logger/    Application logging
internal/models/    Go structs that mirror the database tables
db/migrations/      Numbered SQL migrations that define the schema
docker/             Docker Compose file for Postgres and LocalStack
docs/learn/         Stage-by-stage lessons explaining how it's built
.env.example        Template for local configuration
.golangci.yml       Linter configuration
Makefile            Build, run, lint, Docker and migration commands
```
