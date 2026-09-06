# Backend

Run the development stack from the repository root:

```sh
make prepare
make dev
```

Compose applies migrations automatically at startup. After migrations complete, normal backend startup automatically inserts the configured `ADMIN_TELEGRAM_ID` only if that Telegram ID is absent. Existing rows—including disabled administrators—are left unchanged. This replaces the removed `bootstrap-admin` command.

To manage migrations manually:

```sh
make migrate-up
make migrate-down
```
