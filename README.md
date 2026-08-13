# Crypto Scanner

Crypto Scanner is a monorepo for a Telegram Mini App and its Go backend. The
backend screens Binance Spot markets and exposes the application API. The
frontend will be added separately under `frontend/`.

## Repository layout

```text
.
├── backend/       # Go service, migrations, Dockerfile, and backend tooling
├── docs/          # Repository-wide agent and architecture documentation
├── compose.yaml   # Local stack orchestration
├── .env.example   # Shared local environment template
└── README.md
```

## Backend

Run backend checks from the repository root:

```sh
make -C backend check
```

See [`backend/README.md`](backend/README.md) for backend development, database,
and integration-test instructions.

## Local stack

Create the shared local environment file, then follow the initialization and
startup instructions in the backend guide:

```sh
cp .env.example .env
```

See [`backend/README.md`](backend/README.md#complete-stack-in-docker-compose).
Once started, the API liveness endpoint is
<http://127.0.0.1:8080/health/live>.

The `frontend/` directory is intentionally absent from this checkpoint. It will
be created when frontend development begins.
