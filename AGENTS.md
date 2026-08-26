# Crypto Scanner

Telegram Mini App for scanning cryptocurrency markets and analyzing instruments.

## Project structure

- `backend/`: Go HTTP API, Telegram authentication, market synchronization, analysis criteria, PostgreSQL storage, migrations, and service commands.
- `frontend/`: React Telegram Mini App built with Vite and TanStack Router.
- `docs/agents/`: repository-specific configuration consumed by engineering skills.
- `compose.yaml`: complete local development stack.
- `compose.production.yaml`: production deployment stack.

Instructions in `frontend/AGENTS.md` also apply when working under `frontend/`.

## Technology

- Backend: Go 1.26, PostgreSQL 18, pgx, sqlc, golang-migrate, Binance connector, and CoinGecko.
- Frontend: React 19, TypeScript 6, Vite 8, TanStack Router and Query, Mantine, Biome, and Vitest.
- Infrastructure: Docker Compose, GitHub Actions, GHCR, and Lefthook.

## Development

Copy `.env.example` to `.env`, then run:

```sh
make prepare
make dev
```

The development stack starts PostgreSQL, applies migrations, bootstraps the configured Telegram administrator, starts the backend on `127.0.0.1:8080`, and serves the frontend on `127.0.0.1:3000`.

Use `make migrate-up` and `make migrate-down` for manual migration control.

## Verification

Run the complete repository checks with:

```sh
make check
```

Backend-only verification:

```sh
go -C backend vet ./...
go -C backend test ./...
```

Frontend-only verification:

```sh
npm -C frontend run quality
npm -C frontend run test
npm -C frontend run build
```

Run the checks relevant to changed code before finishing. Use `make check` for cross-cutting changes.

## Generated files

- Do not edit `backend/internal/storage/postgres/sqlc/*.go` directly. Edit migrations or SQL queries and regenerate with sqlc.
- Do not edit `frontend/src/routeTree.gen.ts` directly. It is generated from files under `frontend/src/routes/`.

## Configuration and secrets

- Root `.env` and `.env.local` files are local and gitignored.
- Never commit or print Telegram tokens, generated Telegram init data, database passwords, or API keys.
- Keep public defaults and documentation in `.env.example`.
- Frontend API requests use relative `/api` and `/health` paths; Vite proxies them to the backend during development.

## Agent skills

### Issue tracker

Issues are tracked in this repository's GitHub Issues. See `docs/agents/issue-tracker.md`.

### Triage labels

Triage uses the five default canonical labels. See `docs/agents/triage-labels.md`.

### Domain docs

This repository uses a single-context domain documentation layout. See `docs/agents/domain.md`.
