# 09 — Calculate candle-range percentiles

**What to build:** Provide a deterministic, exchange-independent analyzer for candle-range percentiles that later HTTP slices can use without duplicating statistical behavior.

**Blocked by:** 01 — Initialize the Go project

**Status:** ready-for-agent

- [ ] Each candle range is calculated as `((high - low) / open) * 100`.
- [ ] Non-positive open values are rejected as invalid data rather than silently skipped.
- [ ] Percentiles use Hyndman–Fan type 7 linear interpolation for the full 0–100 range.
- [ ] A single input value produces that value for every percentile.
- [ ] Comparisons retain full precision and four-decimal rounding occurs only at response formatting time.
- [ ] The analyzer reports insufficient history when fewer candles than requested are supplied.
- [ ] The analyzer has no dependency on HTTP, PostgreSQL, Binance, Telegram, or generated adapter types.
- [ ] Unit examples and edge cases from the specification pass deterministically.
