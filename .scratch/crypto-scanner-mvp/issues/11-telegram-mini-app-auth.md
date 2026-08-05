# 11 — Authenticate Telegram Mini App users

**What to build:** Protect business HTTP handlers with Telegram Mini App identity verification and the repository's enabled-user allowlist.

**Blocked by:** 05 — Bootstrap the administrator user

**Status:** ready-for-agent

- [ ] The middleware accepts only the exact `tma` authorization scheme with raw Telegram Mini App init data.
- [ ] Telegram signature verification uses the configured bot token and rejects tampering.
- [ ] Missing or malformed user data and expired authentication dates return `401 unauthenticated`.
- [ ] Unknown or disabled Telegram users return `403 access_denied`.
- [ ] An enabled user is attached to request context for downstream handlers.
- [ ] The maximum init-data age follows configuration.
- [ ] Raw init data, authorization headers, and bot secrets never appear in logs or error responses.
- [ ] Authentication behavior is verified using deterministic signed fixtures and store-backed authorization cases.
