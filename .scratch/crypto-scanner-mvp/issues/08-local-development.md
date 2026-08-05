# 08 — Configure local development

**What to build:** Configure a predictable local development process with two supported modes: run the complete application stack through Docker Compose, or run only PostgreSQL through Docker Compose and start the Go service on the host.

**Blocked by:** 07 — Backfill closed daily candles

**Status:** resolved

- [x] A complete `.env.example` is provided, and one local untracked `.env` contains all values used by Compose, PostgreSQL, migrations, bootstrap, and the Go service.
- [x] The developer fills `.env` once; Compose and development commands load it automatically without additional environment files or manual exports.
- [x] `docker compose up` starts PostgreSQL and the application as a working stack, including the required database initialization.
- [x] PostgreSQL can also be started by itself, and a documented Make command runs the Go service on the host against that same database.
- [x] PostgreSQL has a healthcheck, a persistent named volume, and a loopback port available to the host-run service.
- [x] Migration and administrator-bootstrap commands work against the Compose-managed database in both development modes.
- [x] Both modes serve `GET /health/live`, preserve data across normal restarts, and have documented start, stop, log, and reset commands.
- [x] The README describes both workflows. Production deployment and live Telegram credentials are out of scope.

## Answer

Added a non-root multi-stage Go image and a Compose stack that sequences a
healthy PostgreSQL service, migrations, administrator bootstrap, and the HTTP
application. `.env.example`, the Makefile, and the README now define one local
configuration source and document complete-stack and host-run workflows,
including lifecycle, log, migration, bootstrap, and destructive reset commands.

Verified both modes against `GET /health/live`, verified the administrator row
survives container recreation through the named volume, and ran the full Go
test, vet, and build checks.
