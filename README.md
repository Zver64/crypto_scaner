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
make dependencies # download the versions recorded in go.mod and go.sum
make build        # build bin/crypto-scanner
make test         # run all tests
make vet          # run Go's static checks
make generate     # run registered go:generate directives
make sqlc-version # run the pinned sqlc tool and print its version
make tidy         # normalize go.mod and go.sum
make check        # test, vet, and build
make migrate-up   # migrate DATABASE_URL to the current schema version
make migrate-down # roll the current schema back to zero
make bootstrap-admin # explicitly create or re-enable ADMIN_TELEGRAM_ID
make dev-up       # build and run the complete Compose stack
make dev-db       # run only PostgreSQL for host development
make dev-run      # migrate, bootstrap, and run the Go service on the host
make dev-stop     # stop the Compose services without deleting data
make dev-down     # stop and remove containers without deleting data
make dev-logs     # follow logs from the Compose services
make dev-reset    # remove containers and permanently delete local DB data
make clean        # remove Go build state and the local binary
```

The `generate` target is intentionally the standard `go generate ./...` entry
point. Packages that own generated code add their directives alongside the
source inputs. The `sqlc` executable is pinned with Go's native tool directive
and is available reproducibly as `go tool sqlc`.

## Local development

Create the single local environment file once and replace its placeholder
credentials. Keep `POSTGRES_PASSWORD` URL-safe because the local tooling builds
a PostgreSQL URL from it. Compose reads `.env` automatically, and the Makefile
includes it for host commands; no shell exports or second environment file are
needed. `.env` is ignored by Git.

```sh
cp .env.example .env
```

`PUBLIC_BASE_URL` and the Telegram values are configuration placeholders for
later features. They must be syntactically valid locally, but live Telegram
credentials are not required for the current liveness-only server.

### Complete stack in Docker Compose

Run PostgreSQL, migrations, administrator bootstrap, and the application:

```sh
make dev-up
```

Plain `docker compose up` also starts the stack and builds the image when it is
absent; the Make target adds `--build` so source changes are picked up.
PostgreSQL becomes healthy before migrations run; the application starts only
after migration and bootstrap complete. Check the service at
<http://127.0.0.1:8080/health/live>.

Use `make dev-up-detached` for background containers, `make dev-logs` to follow
their logs, `make dev-stop` to stop them, and `make dev-down` to remove the
containers. The named PostgreSQL volume survives all of these normal restarts.

### PostgreSQL in Compose, Go on the host

Start only PostgreSQL, then run the host service. `dev-run` applies migrations
and bootstraps the configured administrator before it starts the server:

```sh
make dev-db
make dev-run
```

The host service uses the same Compose-managed database through its loopback
port and serves <http://127.0.0.1:8080/health/live>. Stop the Go process with
Ctrl-C. The same `make dev-logs`, `make dev-stop`, and `make dev-down` commands
manage PostgreSQL. `make migrate-up`, `make migrate-down`, and
`make bootstrap-admin` also load `.env` automatically and target this database.

To intentionally delete all local PostgreSQL data and return to a clean state:

```sh
make dev-reset
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
