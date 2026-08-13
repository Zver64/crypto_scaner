# HTTP request collection

This directory contains requests for every HTTP endpoint exposed by the
backend. The files use the JetBrains HTTP format supported by
[Kulala.nvim](https://github.com/mistweaverco/kulala.nvim).

## Setup

Initialize and start the backend using the
[backend development instructions](../README.md#complete-stack-in-docker-compose).

For authenticated analysis requests, create the ignored private environment
file:

```sh
cp backend/http/http-client.private.env.json.example \
  backend/http/http-client.private.env.json
```

Open `backend/http/http-client.private.env.json` and replace the placeholder
with the exact value of `window.Telegram.WebApp.initData` from the Mini App.
The Telegram user must exist and be enabled in the application database. The
manual bootstrap script enables the administrator configured by
`ADMIN_TELEGRAM_ID`:

```sh
make -C backend bootstrap-admin
```

Open either `.http` file in Neovim, place the cursor within a request, and use
Kulala's **Send request** action. Select the `default` environment if another
Kulala environment is active.

## Files

- `health.http` covers liveness and readiness.
- `analysis.http` covers symbol analysis, market search, authentication, and
  argument-validation usage.
- `http-client.env.json` contains safe committed defaults.
- `http-client.private.env.json` contains local Telegram credentials and must
  never be committed.

All endpoints return an `X-Request-ID` response header. API errors use an
`error.code`, `error.message`, and `request_id` JSON envelope. The business
endpoints may return `invalid_argument`, `unauthenticated`, `access_denied`,
`symbol_not_found`, `insufficient_data`, `market_data_unavailable`, or
`internal_error` depending on the request and service state.
