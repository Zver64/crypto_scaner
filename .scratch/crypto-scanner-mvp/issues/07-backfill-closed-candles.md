# 07 — Backfill closed daily candles

**What to build:** Populate every active Binance Spot/USDT instrument with usable historical data while excluding the currently forming daily candle.

**Blocked by:** 06 — Synchronize the Binance Spot instrument catalog

**Status:** resolved

- [x] An instrument with no history receives up to the latest 30 daily UTC candles.
- [x] Only candles whose close time precedes the synchronization start are stored.
- [x] A newly listed instrument with fewer than 30 closed candles succeeds with its available history.
- [x] Reprocessing the same Binance response creates no duplicate candles.
- [x] Stored price and volume values preserve the precision supplied by Binance.
- [x] A permanent failure for one instrument does not prevent attempts for remaining instruments.
- [x] Successful rows remain valid when another instrument fails.
- [x] The synchronization outcome and per-run totals are visible through state and structured logs.

## Answer

Added closed daily-kline retrieval through the official Binance connector and
initial backfill for active instruments without history. Requests are capped at
the preceding UTC day and responses are filtered against the synchronization
start, so forming candles are never stored. Short histories succeed, candle
upserts remain idempotent, and per-instrument failures are aggregated only after
all active instruments have been attempted, preserving successful writes.

Durable synchronization state now records the run outcome and latest closed
open time, while structured logs report the profile, outcome, instrument totals,
and written candle rows. Validated with adapter fixtures, synchronization seam
tests, focused race tests, the full suite, vet, build, formatting, and diff
checks. The guarded real-PostgreSQL suite was not rerun; its existing contract
covers candle upsert idempotency and numeric round-tripping.
