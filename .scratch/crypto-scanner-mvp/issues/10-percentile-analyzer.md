# 10 — Calculate candle-range percentiles

**What to build:** Provide a deterministic, exchange-independent analyzer for candle-range percentiles that later HTTP slices can use without duplicating statistical behavior.

**Blocked by:** 01 — Initialize the Go project

**Status:** resolved

- [x] Each candle range is calculated as `((high - low) / open) * 100`.
- [x] Non-positive open values are rejected as invalid data rather than silently skipped.
- [x] Percentiles use Hyndman–Fan type 7 linear interpolation for the full 0–100 range.
- [x] A single input value produces that value for every percentile.
- [x] Comparisons retain full precision and four-decimal rounding occurs only at response formatting time.
- [x] The analyzer reports insufficient history when fewer candles than requested are supplied.
- [x] The analyzer has no dependency on HTTP, PostgreSQL, Binance, Telegram, or generated adapter types.
- [x] Unit examples and edge cases from the specification pass deterministically.

## Answer

Implemented an exchange-independent `PercentileAnalyzer` with bounded input
validation, latest-period selection, type 7 interpolation, full-precision
results, coverage metadata, and typed insufficient-history reporting. Public
API behavior is covered by deterministic unit tests derived from the
specification.
