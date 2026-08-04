# 05 — Bootstrap the administrator user

**What to build:** Let an operator explicitly create or re-enable the initial Telegram administrator without coupling user mutation to ordinary server startup.

**Blocked by:** 04 — Add PostgreSQL stores and database readiness

**Status:** ready-for-agent

- [ ] A standalone command reads and validates `ADMIN_TELEGRAM_ID`.
- [ ] The command creates an enabled user when the Telegram ID is absent.
- [ ] Running it again is idempotent and does not create duplicate users.
- [ ] Running it for an existing disabled administrator re-enables that user.
- [ ] The command does not modify unrelated users.
- [ ] Normal server startup never silently inserts, enables, or rewrites users.
