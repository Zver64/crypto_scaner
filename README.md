# Crypto Scanner

Telegram Mini App: Go, React and PostgreSQL.

```sh
cp .env.example .env
make prepare
docker compose up
```

The Compose stack waits for PostgreSQL migrations and automatically creates or
re-enables the administrator identified by `ADMIN_TELEGRAM_ID`.

For backend-only development:

```sh
make devbackend
```

Use `make help` to see the remaining dev commands.
