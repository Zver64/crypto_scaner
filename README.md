# Crypto Scanner

Crypto Scanner is a private Go service for screening Binance Spot markets. The
repository is organized as a modular monolith: commands are composed under
`cmd/`, business modules live directly under `internal/`, and external systems
are isolated behind adapter packages.

The module path is `crypto-scanner`. The project specification does not define
a canonical remote repository path, so the module intentionally uses a local
path until one is chosen.

## Prerequisites

- Go 1.26 or newer
- Docker with Docker Compose
- PostgreSQL 18 is supplied by Compose (PostgreSQL 15 or newer is also
  supported by the schema)

## Development commands

Run these commands from the repository root:

```sh
make build        # build bin/crypto-scanner
make test         # run all tests
make vet          # run Go's static checks
make generate     # run registered go:generate directives
make tidy         # normalize go.mod and go.sum
make check        # test, vet, and build
make migrate-up   # migrate DATABASE_URL to the current schema version
make migrate-down # roll the current schema back to zero
make bootstrap-admin # explicitly create or re-enable ADMIN_TELEGRAM_ID
make run          # migrate, bootstrap, and run the Go service on the host
make clean        # remove Go build state and the local binary
```

The `generate` target is the standard `go generate ./...` entry point. Packages
that own generated code add their directives alongside the source inputs. The
`sqlc` executable remains reproducibly available as `go tool sqlc`.

## Local development

Create the single local environment file once and replace its placeholder
credentials. Keep `POSTGRES_PASSWORD` URL-safe because the local tooling builds
a PostgreSQL URL from it. Compose reads `.env` automatically, and the Makefile
includes it for host commands; no shell exports or second environment file are
needed. `.env` is ignored by Git.

```sh
cp .env.example .env
```

Set the Telegram bot token used to validate signed Mini App `initData` and the
administrator Telegram ID before starting the service. The service does not
receive Telegram Bot API updates or register a webhook.

### Complete stack in Docker Compose

Run PostgreSQL, migrations, administrator bootstrap, and the application:

```sh
docker compose up --build
```

Compose starts PostgreSQL and waits for its healthcheck. The application
container then applies migrations, bootstraps the local administrator, and
replaces that setup shell with the Go server. Check the service at
<http://127.0.0.1:8080/health/live>.

Use the ordinary Compose interface to manage the stack:

```sh
docker compose up --build -d --wait
docker compose logs -f
docker compose down
```

The named PostgreSQL volume survives normal container restarts and
`docker compose down`.

### PostgreSQL in Compose, Go on the host

Start only PostgreSQL, then run the host service. `make run` applies migrations
and bootstraps the configured administrator before it starts the server:

```sh
docker compose up -d postgres
make run
```

The host service uses the same Compose-managed database through its loopback
port and serves <http://127.0.0.1:8080/health/live>. Stop the Go process with
Ctrl-C. Use `docker compose logs -f postgres` and `docker compose down` to
inspect and stop PostgreSQL. `make migrate-up`, `make migrate-down`, and
`make bootstrap-admin` also load `.env` automatically and target this database.

To intentionally delete all local PostgreSQL data and return to a clean state:

```sh
docker compose down -v
```

This removes the named database volume; the data cannot be recovered through
the project tooling.

## Database commands

Provision or update the database explicitly when needed:

```sh
make migrate-up
```

`make migrate-up` is safe to repeat. To roll version 1 back to an unmigrated
database, run `make migrate-down`. The rollback drops only the tables and index
owned by this migration. A migration-owned schema is removed when it is empty;
if another operator-created object remains in that schema, the schema and that
object are preserved.

After migrating, bootstrap the initial Telegram administrator explicitly:

```sh
make bootstrap-admin
```

The command reads `DATABASE_URL` and `ADMIN_TELEGRAM_ID`, requires the current
database migration version, and creates or re-enables only that Telegram user.
It is safe to repeat. Normal server startup never creates or enables users.

The PostgreSQL integration test is destructive by design and only runs with an
explicit disposable-database contract:

```sh
CRYPTO_SCANNER_TEST_DATABASE_URL=postgres://scanner@127.0.0.1:5432/scanner_test \
CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1 go test ./internal/migrate
```

The named database must be empty and disposable. Without both variables the
integration test is skipped.

The PostgreSQL store contracts use the same reset guard and additionally cover
user lookup, transactional instrument snapshots, idempotent candle upserts,
checked numeric conversion, and synchronization state:

```sh
CRYPTO_SCANNER_TEST_DATABASE_URL=postgres://scanner@127.0.0.1:5432/scanner_test \
CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1 go test ./internal/storage/postgres -run TestPostgresStoreContracts
```

Run PostgreSQL integration packages serially (`go test -p 1 ./...`) when both
use the same disposable database.

The server validates all settings, connects to PostgreSQL, and requires the
exact current migration version before listening. It never applies migrations
during normal startup. It then emits structured JSON logs and serves
`GET /health/live` on `HTTP_ADDRESS`. `SIGINT` and `SIGTERM` initiate graceful
shutdown bounded by `SHUTDOWN_TIMEOUT`. Synchronization and business endpoints
are added by their respective implementation slices.
