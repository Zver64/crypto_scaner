# Frontend

## Local Telegram authentication

The development server proxies relative `/api` and `/health` requests to the
backend configured by `VITE_API_PROXY_TARGET` in the repository root `.env`.

Generate Telegram development init data from the root
`TELEGRAM_BOT_TOKEN` and `ADMIN_TELEGRAM_ID` values:

```sh
npm run generate:dev-init-data
```

The command writes `TELEGRAM_DEV_INIT_DATA` to the gitignored root `.env.local`.
It does not print the bot token or generated credential. Restart Vite after
regenerating it. During development, the proxy adds the stored credential to
proxied API requests that do not already have an `Authorization` header. A real
Telegram `Authorization` header therefore takes precedence.

The backend accepts the generated value for the duration configured by the
required root `TELEGRAM_INIT_DATA_MAX_AGE` environment variable. The local
example config sets it to 24 hours (`24h`).

## Run

Run from this directory:

```sh
npm ci
npm run dev
```
