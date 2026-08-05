# 08 — Configure local development

**What to build:** Configure a predictable local development process with two supported modes: run the complete application stack through Docker Compose, or run only PostgreSQL through Docker Compose and start the Go service on the host.

**Blocked by:** 07 — Backfill closed daily candles

**Status:** ready-for-agent

- [ ] A complete `.env.example` is provided, and one local untracked `.env` contains all values used by Compose, PostgreSQL, migrations, bootstrap, and the Go service.
- [ ] The developer fills `.env` once; Compose and development commands load it automatically without additional environment files or manual exports.
- [ ] `docker compose up` starts PostgreSQL and the application as a working stack, including the required database initialization.
- [ ] PostgreSQL can also be started by itself, and a documented Make command runs the Go service on the host against that same database.
- [ ] PostgreSQL has a healthcheck, a persistent named volume, and a loopback port available to the host-run service.
- [ ] Migration and administrator-bootstrap commands work against the Compose-managed database in both development modes.
- [ ] Both modes serve `GET /health/live`, preserve data across normal restarts, and have documented start, stop, log, and reset commands.
- [ ] The README describes both workflows. Production deployment and live Telegram credentials are out of scope.
