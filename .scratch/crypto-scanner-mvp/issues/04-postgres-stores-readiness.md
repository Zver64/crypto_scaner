# 04 — Add PostgreSQL stores and database readiness

**What to build:** Connect the running service to PostgreSQL through typed stores so later market, user, and sync behaviors share one verified persistence seam.

**Blocked by:** 03 — Provision PostgreSQL with explicit migrations

**Status:** ready-for-agent

- [ ] Typed database access is generated from explicit SQL and works through `pgx`.
- [ ] User, instrument, candle, and synchronization-state operations required by the specification are available through narrow store interfaces.
- [ ] Database-generated types do not leak into domain or application interfaces.
- [ ] Candle upserts are idempotent on instrument, interval, and open time.
- [ ] Instrument snapshot application updates active and inactive instruments transactionally.
- [ ] Numeric exchange values are checked during conversion before reaching analysis code.
- [ ] `GET /health/ready` returns HTTP 503 when PostgreSQL or migrations are unavailable, and also reports that no successful market sync exists yet.
- [ ] Representative store behavior is verified against real PostgreSQL.
