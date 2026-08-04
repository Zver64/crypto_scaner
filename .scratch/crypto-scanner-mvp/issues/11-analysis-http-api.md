# 11 — Expose authenticated percentile analysis

**What to build:** Deliver both user-facing screening operations through the versioned authenticated HTTP API using only synchronized PostgreSQL data.

**Blocked by:** 07 — Backfill closed daily candles; 09 — Calculate candle-range percentiles; 10 — Authenticate Telegram Mini App users

**Status:** ready-for-agent

- [ ] An enabled Telegram user can request a percentile for one active symbol with a valid period and percentile.
- [ ] The one-symbol response contains the calculated range, candle count, and UTC coverage.
- [ ] Unknown or inactive symbols return `404 symbol_not_found`.
- [ ] A one-symbol request with insufficient history returns `409 insufficient_data` with required and available counts.
- [ ] An enabled user can search all active instruments using period, percentile, and minimum range.
- [ ] Market search includes exactly values meeting the unrounded threshold.
- [ ] Search results sort by range descending and symbol ascending, and include analyzed, matched, and insufficient-data counts.
- [ ] Invalid or out-of-range query parameters return the canonical `400 invalid_argument` envelope.
- [ ] When no successful market dataset exists, analysis returns `503 market_data_unavailable`.
- [ ] Analysis requests never call Binance and calculated results are neither cached nor persisted.
- [ ] Concurrent analysis requests do not take an application-wide lock.
