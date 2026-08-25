# Repository guide

## Setup and verification

- First run: `cp .env.example .env`, then `make prepare`; `make dev` starts the full stack with Docker Compose watch mode.
- `make check` is the full gate: Go formatting, vet, and tests, then frontend Biome/typecheck and Vitest.
- Focus a Go test with `go -C backend test ./path/to/package -run '^TestName$'`.
- Focus a frontend test with `npm -C frontend exec vitest run path/to/file.test.ts -t 'test name'`.
- `make check` does not build the frontend bundle; use `npm -C frontend run build` when build behavior changes.
- Manual migration commands are `make migrate-up` and `make migrate-down`.

## Generated code

- TanStack Router generates `frontend/src/routeTree.gen.ts`; do not edit it. After route changes, run `npm -C frontend run generate-routes` before checks.
- sqlc output lives in `backend/internal/storage/postgres/sqlc`; after changing its schema/query contracts, run `go -C backend generate ./internal/storage/postgres`.

## Analysis criteria

- Backend filters are called criteria. Keep each criterion in its own `backend/internal/analysis/criteria/<name>/` package, with its implementation and colocated tests.
- A new criterion provides an `analysis.Factory` and implements `analysis.Criterion`; registration is explicit in `backend/cmd/crypto-scanner/main.go`, not automatic package discovery.

## Runtime boundaries

- `backend/cmd/crypto-scanner` wires PostgreSQL, Binance synchronization, analysis, Telegram authentication, HTTP routing, and scheduling. `backend/cmd/migrate` and `backend/cmd/bootstrap-admin` are separate operational binaries.
- HTTP routes and readiness are in `backend/internal/httpapi`; readiness requires the database, current migrations, and at least one successful market sync.
- The frontend starts at `frontend/src/main.tsx`. Its Vite server proxies `/api` and `/health` to `VITE_API_PROXY_TARGET`.
- Compose starts PostgreSQL, migrations, admin bootstrap, backend, then frontend; do not duplicate migration/bootstrap work in normal `make dev` startup.

## Environment and tests

- Frontend development generates Telegram init data from root `.env` into gitignored `.env.local`. After manual `npm -C frontend run generate:dev-init-data`, restart Vite so the proxy reloads it.
- PostgreSQL integration tests skip unless `CRYPTO_SCANNER_TEST_DATABASE_URL` names a disposable empty database and `CRYPTO_SCANNER_TEST_DATABASE_RESET_OK=1`; these tests drop/reset schemas.
- Before frontend edits, follow the matching TanStack Intent command catalog in `frontend/AGENTS.md`.

## Project workflow

- Local specs and issues live under `.scratch/<feature>/`; follow `docs/agents/issue-tracker.md` and its triage roles in `docs/agents/triage-labels.md`.
- Read `CONTEXT.md` before changing domain terminology; surface conflicts with any `docs/adr/` decision instead of silently replacing it.
- Never emit Go binaries into the repository root; use `backend/bin/` for repository-local build artifacts.
