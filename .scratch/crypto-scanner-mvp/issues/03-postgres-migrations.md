# 03 — Provision PostgreSQL with explicit migrations

**What to build:** Give an operator a standalone migration command that creates the complete MVP database schema without allowing normal server startup to mutate it.

**Blocked by:** 02 — Add configuration, logging, and process lifecycle

**Status:** resolved

- [x] A fresh PostgreSQL database can be upgraded from zero to the complete schema defined in the specification.
- [x] The migration creates the `app` and `binance_spot` schemas and all required tables, constraints, keys, and indexes.
- [x] Running the migration command twice is safe and leaves the schema at the expected version.
- [x] A corresponding rollback removes only objects owned by this migration.
- [x] Normal server startup detects a missing or outdated schema and fails without applying changes.
- [x] Database errors are returned with useful context without exposing credentials.

## Answer

Added embedded, handwritten version-1 up/down migrations and a standalone
`cmd/migrate` command exposed through `make migrate-up` and `make migrate-down`.
Migration execution is transactional and advisory-locked, repeat-safe, and the
rollback preserves non-migration objects. Normal service startup now pings
PostgreSQL and requires the exact current schema version before opening its HTTP
listener; it never applies migrations. Database connection failures are
contextualized and credentials are redacted.

Validated against an isolated PostgreSQL 18 cluster with zero → up → up → down,
catalog checks for every MVP table/constraint/index, rollback ownership, failed
startup on an unmigrated schema, successful startup on version 1, full tests,
race detection, vet, build, and `make check`.
