# 03 — Provision PostgreSQL with explicit migrations

**What to build:** Give an operator a standalone migration command that creates the complete MVP database schema without allowing normal server startup to mutate it.

**Blocked by:** 02 — Add configuration, logging, and process lifecycle

**Status:** ready-for-agent

- [ ] A fresh PostgreSQL database can be upgraded from zero to the complete schema defined in the specification.
- [ ] The migration creates the `app` and `binance_spot` schemas and all required tables, constraints, keys, and indexes.
- [ ] Running the migration command twice is safe and leaves the schema at the expected version.
- [ ] A corresponding rollback removes only objects owned by this migration.
- [ ] Normal server startup detects a missing or outdated schema and fails without applying changes.
- [ ] Database errors are returned with useful context without exposing credentials.
