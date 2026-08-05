# 12 — Expose authenticated percentile analysis

**What to build:** Deliver both user-facing screening operations through the versioned authenticated HTTP API using only synchronized PostgreSQL data.

**Blocked by:** 09 — Run incremental synchronization; 10 — Calculate candle-range percentiles; 11 — Authenticate Telegram Mini App users

**Status:** resolved

- [x] An enabled Telegram user can request a percentile for one active symbol with a valid period and percentile.
- [x] The one-symbol response contains the calculated range, candle count, and UTC coverage.
- [x] Unknown or inactive symbols return `404 symbol_not_found`.
- [x] A one-symbol request with insufficient history returns `409 insufficient_data` with required and available counts.
- [x] An enabled user can search all active instruments using period, percentile, and minimum range.
- [x] Market search includes exactly values meeting the unrounded threshold.
- [x] Search results sort by range descending and symbol ascending, and include analyzed, matched, and insufficient-data counts.
- [x] Invalid or out-of-range query parameters return the canonical `400 invalid_argument` envelope.
- [x] When no successful market dataset exists, analysis returns `503 market_data_unavailable`.
- [x] Analysis requests never call Binance and calculated results are neither cached nor persisted.
- [x] Concurrent analysis requests do not take an application-wide lock.

## Answer

Added an application analysis service backed only by synchronized market-store
reads and exposed both percentile operations through authenticated `/api/v1`
HTTP routes. The API validates canonical arguments and errors, rounds only JSON
output, retains unrounded market filtering and deterministic sorting, reports
coverage/count metadata, and has no cache, persistence writes, Binance calls, or
application-wide analysis lock.
