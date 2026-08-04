# 05 — Bootstrap the administrator user

**What to build:** Let an operator explicitly create or re-enable the initial Telegram administrator without coupling user mutation to ordinary server startup.

**Blocked by:** 04 — Add PostgreSQL stores and database readiness

**Status:** resolved

- [x] A standalone command reads and validates `ADMIN_TELEGRAM_ID`.
- [x] The command creates an enabled user when the Telegram ID is absent.
- [x] Running it again is idempotent and does not create duplicate users.
- [x] Running it for an existing disabled administrator re-enables that user.
- [x] The command does not modify unrelated users.
- [x] Normal server startup never silently inserts, enables, or rewrites users.

## Answer

Added `cmd/bootstrap-admin`, backed by a command-specific configuration loader,
verified PostgreSQL connection, and an explicit sqlc upsert that changes only
the configured Telegram user. Repeating the command for an enabled user is a
true no-op; a disabled matching user is re-enabled and receives a new
`updated_at` value. Real-PostgreSQL integration tests cover create, repeat,
re-enable, unrelated-user preservation, schema verification, and prove that
normal server startup leaves users unchanged.
