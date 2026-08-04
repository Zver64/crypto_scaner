# 07 — Backfill closed daily candles

**What to build:** Populate every active Binance Spot/USDT instrument with usable historical data while excluding the currently forming daily candle.

**Blocked by:** 06 — Synchronize the Binance Spot instrument catalog

**Status:** ready-for-agent

- [ ] An instrument with no history receives up to the latest 30 daily UTC candles.
- [ ] Only candles whose close time precedes the synchronization start are stored.
- [ ] A newly listed instrument with fewer than 30 closed candles succeeds with its available history.
- [ ] Reprocessing the same Binance response creates no duplicate candles.
- [ ] Stored price and volume values preserve the precision supplied by Binance.
- [ ] A permanent failure for one instrument does not prevent attempts for remaining instruments.
- [ ] Successful rows remain valid when another instrument fails.
- [ ] The synchronization outcome and per-run totals are visible through state and structured logs.
