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
- PostgreSQL 18 (PostgreSQL 15 or newer is also supported by the schema)

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
make clean        # remove Go build state and the local binary
```

The `generate` target is intentionally the standard `go generate ./...` entry
point. Packages that own generated code add their directives alongside the
source inputs. The `sqlc` executable is pinned with Go's native tool directive
and is available reproducibly as `go tool sqlc`.

## Running the service

Copy `.env.example` to `.env`, replace every placeholder, and inject the values
into the process environment. The application intentionally does not load
`.env` files itself. `PUBLIC_BASE_URL` is optional for the server and is used by
the standalone webhook registration command added in a later slice.

```sh
set -a
. ./.env
set +a
go run ./cmd/crypto-scanner
```

Provision or update the database explicitly before starting the service:

```sh
make migrate-up
```

`make migrate-up` is safe to repeat. To roll version 1 back to an unmigrated
database, run `make migrate-down`. The rollback drops only the tables and index
owned by this migration. A migration-owned schema is removed when it is empty;
if another operator-created object remains in that schema, the schema and that
object are preserved.

The PostgreSQL integration test is destructive by design and only runs with an
explicit disposable-database contract:

```sh
CRYPTO_SCANNER_TEST_DATABASE_URL=postgres://scanner@127.0.0.1:5432/scanner_test \
CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1 go test ./internal/migrate
```

The named database must be empty and disposable. Without both variables the
integration test is skipped.

The server validates all settings, connects to PostgreSQL, and requires the
exact current migration version before listening. It never applies migrations
during normal startup. It then emits structured JSON logs and serves
`GET /health/live` on `HTTP_ADDRESS`. `SIGINT` and `SIGTERM` initiate graceful
shutdown bounded by `SHUTDOWN_TIMEOUT`. Synchronization and business endpoints
are added by their respective implementation slices.
