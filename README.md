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

The process validates all settings before listening, emits structured JSON logs,
and serves `GET /health/live` on `HTTP_ADDRESS`. `SIGINT` and `SIGTERM` initiate
graceful shutdown bounded by `SHUTDOWN_TIMEOUT`. Storage, synchronization, and
business endpoints are added by their respective implementation slices.
