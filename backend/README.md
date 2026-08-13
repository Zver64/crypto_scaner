# Crypto Scanner Backend

Crypto Scanner is a private Go service for screening Binance Spot markets. The
backend is organized as a modular monolith: commands are composed under
`cmd/`, business modules live directly under `internal/`, and external systems
are isolated behind adapter packages.

The module path is `crypto-scanner`. The project specification does not define
a canonical remote repository path, so the module intentionally uses a local
path until one is chosen.

## Prerequisites

- Go 1.26 or newer
- Docker with Docker Compose
- Air for Go hot reload
- PostgreSQL client (`psql`) for the administrator bootstrap script
- PostgreSQL 18 is supplied by Compose (PostgreSQL 15 or newer is also
  supported by the schema)

## Development commands

Run these commands from the repository root:

```sh
make -C backend build        # build backend/bin/crypto-scanner
make -C backend build-all    # build the service and migration command
make -C backend test         # run all tests
make -C backend vet          # run Go's static checks
make -C backend generate     # run registered go:generate directives
make -C backend tidy         # normalize go.mod and go.sum
make -C backend check        # test, vet, and build
make -C backend migrate-up   # migrate DATABASE_URL to the current schema version
make -C backend migrate-down # roll the current schema back to zero
make -C backend bootstrap-admin # explicitly create or re-enable ADMIN_TELEGRAM_ID
make -C backend run          # run the Go service on the host
make -C backend dev          # run the Go service with hot reload
make -C backend clean        # remove Go build state and local binaries
```

The `generate` target is the standard `go generate ./...` entry point. Packages
that own generated code add their directives alongside the source inputs. The
`sqlc` executable remains reproducibly available as `go tool sqlc`.

## Local development

Create the shared local environment file at the repository root and replace
its placeholder credentials. Keep `POSTGRES_PASSWORD` URL-safe because the
local tooling builds a PostgreSQL URL from it. Compose reads the root `.env`
automatically, and the backend Makefile includes `../.env` for host commands;
no shell exports or second environment file are needed. `.env` is ignored by
Git.

```sh
cp .env.example .env
```

Set the Telegram bot token used to validate signed Mini App `initData`. Set the
administrator Telegram ID before running the manual bootstrap script; the
server itself does not read that setting. The service does not receive Telegram
Bot API updates or register a webhook.

### Hot-reload development

Install Air globally once so that the `air` command is available on `PATH`:

```sh
go install github.com/air-verse/air@latest
```

The command installs Air into `GOBIN`, or into `GOPATH/bin` when `GOBIN` is
unset. Ensure that directory is included in your `PATH`.

When initializing a new development database, start PostgreSQL, apply the
migrations, and explicitly create the administrator from the repository root:

```sh
docker compose up -d --wait postgres
make -C backend migrate-up
make -C backend bootstrap-admin
```

Then start the hot-reload server:

```sh
make -C backend dev
```

Ordinary sessions only require PostgreSQL and `make -C backend dev`. Run
`migrate-up` after pulling or adding migrations. Run `bootstrap-admin` only
when initializing a database or intentionally re-enabling the configured
administrator.

Air rebuilds and restarts the service after Go file changes. The development
binary stays in the ignored `backend/bin/air/` directory; its build command is
delegated to the backend Makefile. Air sends an interrupt on rebuild and waits
for the configured `SHUTDOWN_TIMEOUT` before forcing termination. Press Ctrl-C
to stop the service; PostgreSQL can remain running between development
sessions. If the Compose `app` service is already running, stop it first with
`docker compose stop app` so it does not occupy port 8080.

### Complete stack in Docker Compose

Build the images and start PostgreSQL first:

```sh
docker compose build
docker compose up -d --wait postgres
```

When initializing a new database, apply migrations and explicitly bootstrap
the administrator:

```sh
docker compose run --rm app /usr/local/bin/migrate up
make -C backend bootstrap-admin
```

Then start the application:

```sh
docker compose up -d --wait app
```

Ordinary application restarts never migrate the database or enable users. Run
the migration command after pulling or adding migrations, and run the bootstrap
command only when intentionally creating or re-enabling the administrator.
Check the service at <http://127.0.0.1:8080/health/live>.

Use the ordinary Compose interface to manage the stack:

```sh
docker compose up --build -d --wait
docker compose logs -f
docker compose down
```

The named PostgreSQL volume survives normal container restarts and
`docker compose down`.

### PostgreSQL in Compose, Go on the host

From the repository root, start only PostgreSQL and then run the host service:

```sh
docker compose up -d postgres
make -C backend run
```

Initialize or update the database beforehand with the explicit `migrate-up`
and `bootstrap-admin` commands described above. The `run` target never changes
the database schema or enables users.

The host service uses the same Compose-managed database through its loopback
port and serves <http://127.0.0.1:8080/health/live>. Stop the Go process with
Ctrl-C. Use `docker compose logs -f postgres` and `docker compose down` to
inspect and stop PostgreSQL. The backend `migrate-up`, `migrate-down`, and
`bootstrap-admin` targets also load the root `.env` automatically and target
this database.

To intentionally delete all local PostgreSQL data and return to a clean state:

```sh
docker compose down -v
```

This removes the named database volume; the data cannot be recovered through
the project tooling.

## Database commands

Provision or update the database explicitly when needed:

```sh
make -C backend migrate-up
```

`make -C backend migrate-up` is safe to repeat. To roll version 1 back to an
unmigrated database, run `make -C backend migrate-down`. The rollback drops
only the tables and index owned by this migration. A migration-owned schema is
removed when it is empty; if another operator-created object remains in that
schema, the schema and that object are preserved.

After migrating, bootstrap the initial Telegram administrator explicitly:

```sh
make -C backend bootstrap-admin
```

The target passes `ADMIN_TELEGRAM_ID` safely to the lightweight
`scripts/bootstrap-admin.sql` script and connects to the Compose-managed
PostgreSQL database using the configured `POSTGRES_*` values. The script
creates or re-enables only that Telegram user and is safe to repeat. Normal
server startup never creates or enables users, and the bootstrap script is not
included in the application image.

The PostgreSQL integration test is destructive by design and only runs with an
explicit disposable-database contract:

```sh
CRYPTO_SCANNER_TEST_DATABASE_URL=postgres://scanner@127.0.0.1:5432/scanner_test \
CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1 go -C backend test ./internal/migrate
```

The named database must be empty and disposable. Without both variables the
integration test is skipped.

The PostgreSQL store contracts use the same reset guard and additionally cover
user lookup, transactional instrument snapshots, idempotent candle upserts,
checked numeric conversion, and synchronization state:

```sh
CRYPTO_SCANNER_TEST_DATABASE_URL=postgres://scanner@127.0.0.1:5432/scanner_test \
CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1 go -C backend test ./internal/storage/postgres -run TestPostgresStoreContracts
```

Run PostgreSQL integration packages serially (`go -C backend test -p 1 ./...`)
when both use the same disposable database.

The server validates all settings, connects to PostgreSQL, and requires the
exact current migration version before listening. It never applies migrations
during normal startup. It then emits structured JSON logs and serves
`GET /health/live` on `HTTP_ADDRESS`. `SIGINT` and `SIGTERM` initiate graceful
shutdown bounded by `SHUTDOWN_TIMEOUT`. Synchronization and business endpoints
are added by their respective implementation slices.
