# Crypto Scanner

Crypto Scanner is a Telegram Mini App with a Go backend, a Vite frontend, and
PostgreSQL. Docker Compose manages only the local database; the application
processes run directly on the developer's machine.

## Prerequisites

- Go 1.26 or newer
- Node.js with `npm`
- Docker with Docker Compose

## Quickstart

Create the single local environment file at the repository root and replace its
placeholder values:

```sh
cp .env.example .env
```

Start PostgreSQL from the repository root:

```sh
docker compose up -d --wait
```

Initialize or update the schema, then start the backend:

```sh
cd backend
go run ./cmd/migrate up
go run ./cmd/crypto-scanner
```

The migration command is safe to repeat and is only needed on first setup or
after migration changes. If an initial Telegram administrator is needed, set
`ADMIN_TELEGRAM_ID` in the root `.env` and run
`go run ./cmd/bootstrap-admin` after migrating.

In another terminal, install the locked frontend dependencies once and start
Vite:

```sh
cd frontend
npm ci
npm run dev
```

Compose reads the root `.env` natively. Every backend command above loads the
same file automatically without overriding variables already present in the
process environment. The backend safely builds its local PostgreSQL connection
URL from the `POSTGRES_*` settings; an explicit `DATABASE_URL` takes precedence
when deployment requires one. Vite also reads the root file and proxies
`/health` and `/api` to `VITE_API_PROXY_TARGET`, so browser requests remain
same-origin.

The frontend is available at <http://127.0.0.1:3000>. The backend liveness
endpoint is <http://127.0.0.1:8080/health/live>. For Telegram WebApp testing,
publish the Vite development server through an ephemeral HTTPS tunnel; Vite
accepts the tunnel's varying external hostnames for this development flow.

## Checks

```sh
go -C backend test ./...
go -C backend vet ./...
npm --prefix frontend run check
npm --prefix frontend run build
```

See [`backend/README.md`](backend/README.md) and
[`frontend/README.md`](frontend/README.md) for component-specific details.
