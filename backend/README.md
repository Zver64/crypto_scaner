# Crypto Scanner Backend

The backend is a Go modular monolith. Run its commands from this `backend/`
directory. Each entrypoint automatically loads the repository root `.env` when
it exists; already-set process variables take precedence.

## Run

Start PostgreSQL from the repository root with `docker compose up -d --wait`,
then run:

```sh
go run ./cmd/migrate up
go run ./cmd/crypto-scanner
```

Normal server startup never changes the database schema. Apply migrations on
first setup and after migration changes. A deliberate rollback is available as
`go run ./cmd/migrate down` and can remove application data.

To create or re-enable the Telegram user configured by `ADMIN_TELEGRAM_ID`, run
this after the schema is current:

```sh
go run ./cmd/bootstrap-admin
```

The server listens on `HTTP_ADDRESS` and exposes `GET /health/live`,
`GET /health/ready`, and the authenticated API below `/api/v1`.

## Optional hot reload

[Air](https://github.com/air-verse/air) is optional. If it is installed, run
`air` (or `make dev`). Its temporary binary is written under `backend/bin/air/`.
The canonical command remains `go run ./cmd/crypto-scanner`.

The Makefile contains only thin aliases for building, running, migrating, and
starting Air. It does not read, export, or reconstruct configuration.

Backend commands build the local connection URL from the root `.env`
`POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, and
`POSTGRES_DB` values. Credentials and database names are URL-encoded safely.
For deployments and backwards compatibility, a non-empty `DATABASE_URL`
overrides those individual settings.

## Development checks

```sh
go test ./...
go vet ./...
go generate ./...
go build -o bin/crypto-scanner ./cmd/crypto-scanner
```

PostgreSQL integration tests require the disposable database guard variables
documented in the tests themselves. They are skipped by default. Run packages
serially when multiple integration packages share the same disposable database.
