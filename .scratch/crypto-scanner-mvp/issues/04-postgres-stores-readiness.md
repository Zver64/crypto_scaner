# 04 — Add PostgreSQL stores and database readiness

**What to build:** Connect the running service to PostgreSQL through typed stores so later market, user, and sync behaviors share one verified persistence seam.

**Blocked by:** 03 — Provision PostgreSQL with explicit migrations

**Status:** resolved

- [x] Typed database access is generated from explicit SQL and works through `pgx`.
- [x] User, instrument, candle, and synchronization-state operations required by the specification are available through narrow store interfaces.
- [x] Database-generated types do not leak into domain or application interfaces.
- [x] Candle upserts are idempotent on instrument, interval, and open time.
- [x] Instrument snapshot application updates active and inactive instruments transactionally.
- [x] Numeric exchange values are checked during conversion before reaching analysis code.
- [x] `GET /health/ready` returns HTTP 503 when PostgreSQL or migrations are unavailable, and also reports that no successful market sync exists yet.
- [x] Representative store behavior is verified against real PostgreSQL.

## Answer

Added explicit user, instrument, candle, and sync-state SQL compiled by the
pinned `go tool sqlc` into a PostgreSQL-private generated package. The pgx
adapter maps those rows into narrow `auth.UserStore` and `market.MarketStore`
domain seams, applies instrument snapshots and candle batches transactionally,
keeps candle upserts idempotent, and rejects non-finite or out-of-range numeric
conversions before analysis-facing values are returned. Failed sync updates
preserve the last successful dataset metadata.

Added `GET /health/ready` with independent database, migration, and successful
market-sync checks. It remains unavailable until all three pass and continues
to report ready after a later failed sync when successful data exists.

Validated the store contract against an isolated PostgreSQL 18 cluster,
including disabled-user lookup, snapshot rollback/deactivation/reactivation,
candle replacement and numeric overflow, and sync-state transitions. Also ran
reproducible sqlc generation, the full tests and race tests, vet, build, tidy,
and diff checks.
