# Frontend

## Local Telegram authentication

The development server proxies relative `/api` and `/health` requests to the
backend configured by `VITE_API_PROXY_TARGET` in the repository root `.env`.

Before every `npm run dev`, the frontend generates fresh Telegram development
init data from the root `TELEGRAM_BOT_TOKEN` and `ADMIN_TELEGRAM_ID` values. The
generator writes `TELEGRAM_DEV_INIT_DATA` to the gitignored root `.env.local`
before Vite starts. It does not print the bot token or generated credential.
During development, the proxy adds the stored credential to proxied API requests
that do not already have an `Authorization` header. A real Telegram
`Authorization` header therefore takes precedence.

The init data can still be refreshed without starting Vite:

```sh
npm run generate:dev-init-data
```

The backend accepts the generated value for the duration configured by the
required root `TELEGRAM_INIT_DATA_MAX_AGE` environment variable. The local
example config sets it to 24 hours (`24h`).

## Run

Start PostgreSQL and the backend from the repository root:

```sh
make db-up
make migrate-up
make devbackend
```

Then install frontend dependencies and start Vite from this directory:

```sh
npm ci
npm run dev
```

Vite serves the Mini App at `http://127.0.0.1:3000` and proxies its relative
API and readiness requests to the backend. The header should report `Ready`
before Market Scan or Instrument Analysis can run.

## Verify

Run the complete frontend verification from this directory:

```sh
npm test
npm run quality
npm run build
```

`npm test` runs the pure Vitest suite. `npm run quality` runs Biome checks and
TypeScript typechecking. `npm run build` creates the production bundle in
`dist/`; the generated directory is a local build artifact and is not committed.
