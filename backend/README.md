# Backend

Run PostgreSQL, migrations, and the backend from the repository root:

```sh
make prepare
docker compose up
```

Use `docker compose up --watch` instead for automatic backend rebuilds on file
changes. Start the frontend separately with `npm -C frontend run dev`.

Compose applies migrations automatically at startup. After migrations complete, normal backend startup automatically inserts the configured `ADMIN_TELEGRAM_ID` only if that Telegram ID is absent. Existing rows—including disabled administrators—are left unchanged. This replaces the removed `bootstrap-admin` command.

To manage migrations manually:

```sh
make migrate-up
make migrate-down
```
