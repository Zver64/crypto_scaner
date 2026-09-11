# Crypto Scanner

A Telegram Mini App for scanning cryptocurrency markets and inspecting individual instruments. It combines Binance price history with CoinGecko market-cap data to help find actively moving, sufficiently liquid markets.

## Features

- Scan markets by volatility, lookback period, percentile, and minimum market cap
- Browse leading cryptocurrencies by market cap
- Inspect price history and volatility statistics for an instrument
- Estimate spot-grid parameters from recent market behavior
- Manage application access through a Telegram bot

## Tech stack

- **Backend:** Go, PostgreSQL, pgx, and sqlc
- **Frontend:** React, TypeScript, Vite, TanStack Router and Query, and Mantine
- **Market data:** Binance and CoinGecko
- **Infrastructure:** Docker Compose and GitHub Actions

## Getting started

### Prerequisites

- Docker with Docker Compose
- Go 1.26 or later
- Node.js 24 and npm
- A Telegram bot token and your Telegram user ID

### 1. Configure the application

```sh
git clone https://github.com/Zver64/crypto_scaner.git
cd crypto_scaner
cp .env.example .env
```

Edit `.env` and set at least:

- `TELEGRAM_BOT_TOKEN` — token issued by [BotFather](https://t.me/BotFather)
- `ADMIN_TELEGRAM_ID` — Telegram user ID for the initial administrator
- `POSTGRES_PASSWORD` — password for the local database

The remaining settings have development-friendly defaults. See [`.env.example`](.env.example) for all available options.

### 2. Install dependencies

```sh
make prepare
```

This installs Go and npm dependencies and configures the repository's Git hooks.

### 3. Start the backend

```sh
docker compose up
```

Docker Compose starts PostgreSQL, applies database migrations, and serves the API at `http://127.0.0.1:8080`.

### 4. Start the frontend

In another terminal:

```sh
npm -C frontend run dev
```

Open `http://127.0.0.1:3000`. The development server generates local Telegram authentication data from your `.env` values and proxies API requests to the backend.

## Common commands

| Command | Description |
| --- | --- |
| `make check` | Run backend and frontend checks and tests |
| `npm -C frontend run build` | Create a production frontend build |
| `make migrate-up` | Apply pending database migrations manually |
| `make migrate-down` | Roll back one database migration |

## Project structure

```text
backend/                  Go API, Telegram bot, market synchronization, and migrations
frontend/                 React Telegram Mini App
docs/                     Project documentation
compose.yaml              Local development services
compose.production.yaml   Production services
```

## Production

[`compose.production.yaml`](compose.production.yaml) is a standalone production configuration. Before starting it, provide an `.env` file with production credentials and set `BACKEND_IMAGE` and `FRONTEND_IMAGE` to the container image tags to deploy.

```sh
docker compose -f compose.production.yaml up -d
```

The production frontend listens on `127.0.0.1:8081` by default. Set `WEB_BIND_ADDRESS` to change the bind address, typically when placing the application behind a reverse proxy.
