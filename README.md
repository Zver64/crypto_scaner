# Crypto Scanner

Telegram Mini App: Go, React and PostgreSQL.

```sh
cp .env.example .env
make prepare
make dev
```

Compose starts the development stack in watch mode, applies PostgreSQL migrations
automatically, and creates or re-enables the administrator identified by
`ADMIN_TELEGRAM_ID`.

`make prepare` installs backend and frontend dependencies and the Git hooks.
Run the full verification suite with:

```sh
make check
```

## Migrations

```sh
make migrate-up    # apply migrations manually
make migrate-down  # roll back one migration manually
```
