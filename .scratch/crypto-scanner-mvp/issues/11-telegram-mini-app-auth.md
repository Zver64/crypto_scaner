# 11 — Authenticate Telegram Mini App users

**What to build:** Protect business HTTP handlers with Telegram Mini App identity verification and the repository's enabled-user allowlist.

**Blocked by:** 05 — Bootstrap the administrator user

**Status:** resolved

- [x] The middleware accepts only the exact `tma` authorization scheme with raw Telegram Mini App init data.
- [x] Telegram signature verification uses the configured bot token and rejects tampering.
- [x] Missing or malformed user data and expired authentication dates return `401 unauthenticated`.
- [x] Unknown or disabled Telegram users return `403 access_denied`.
- [x] An enabled user is attached to request context for downstream handlers.
- [x] The maximum init-data age follows configuration.
- [x] Raw init data, authorization headers, and bot secrets never appear in logs or error responses.
- [x] Authentication behavior is verified using deterministic signed fixtures and store-backed authorization cases.

## Answer

Implemented Telegram Mini App HTTP authentication middleware with exact-scheme
parsing, bot-token HMAC verification, configurable freshness validation,
enabled-user authorization, authenticated-user context propagation, and safe
canonical error responses. Added deterministic signed-fixture coverage and a
store-level not-found contract for unknown and disabled users.
