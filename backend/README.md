# Backend

Run PostgreSQL, migrations, and the backend from the repository root:

```sh
make prepare
docker compose up
```

Use `docker compose up --watch` instead for automatic backend rebuilds on file
changes. Start the frontend separately with `npm -C frontend run dev`.

The backend-owned OpenAPI 3.1 contract is at `internal/httpapi/openapi/openapi.yaml`. Run `make generate-backend` after changing it; generated bindings are committed and CI rejects stale output. Development Compose enables Swagger UI at `http://127.0.0.1:8080/docs/` and the raw contract at `/openapi.yaml`. Set `API_DOCS_ENABLED=true` only for local development; production explicitly disables both routes.

Compose applies migrations automatically at startup. After migrations complete, normal backend startup automatically inserts the configured `ADMIN_TELEGRAM_ID` only if that Telegram ID is absent. Existing rows—including disabled administrators—are left unchanged. This replaces the removed `bootstrap-admin` command.

To manage migrations manually:

```sh
make migrate-up
make migrate-down
```
