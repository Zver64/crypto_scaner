# Strategy language

Reference for writing a strategy expression that the scanner imports unchanged.

## Indicators and variable names

- Indicators are TA-Lib functions. Types, parameter order, defaults, and output names are in `backend/internal/indicator/talib/functions_gen.go` (`indicatorType`, `params`, `outputs`). TA-Lib defaults are not obvious: `ema`, `sma`, `max`, and `min` use 30, `roc` 10, `bbands` 5.
- An indicator title is `<interval>-<type>[-parameters]`. Intervals: `h` (1h), `d` (1d), `w` (1w), `m` (1M). Numeric parameters follow in order up to the last non-default one; trailing defaults are dropped: `h-rsi` is RSI 14, `h-rsi-21`, `h-bbands-20-2.5` (upper band), `h-bbands-20-2-2.5` (lower band). The MA type is a number (`0` SMA, `1` EMA, …).
- The candle field (`source`) is spelled as a word and only when it is not `close`: `h-max-12-high`, `h-sma-20-volume`. Fields: `open`, `high`, `low`, `close`, `volume`, `quote_asset_volume`, `trade_count`.
- A variable is the title with `.` → `p` and other characters → `_`: `h-bbands-20-2.5` → `h_bbands_20_2p5`. An indicator with several outputs appends `_<output>`: `h_bbands_20_2p5_upperband`, `h_macd_macdsignal`, `h_stoch_slowk`.
- Candle fields need no indicator: `<interval>_open`, `_high`, `_low`, `_close`, `_volume`, `_quote_volume`, `_trades` (`h_close`).
- A name must spell back exactly one indicator; the import then offers to add indicators that are not configured. Do not invent names or use `__`.

## Expressions

- The result is a boolean: comparisons (`<`, `<=`, `>`, `>=`) of numbers and variables combined with `&&`, `||`, and `!`. Arithmetic: `+ - * /`, `abs(x)`, `mod(a, b)`, `min(…)`, `max(…)` (2–10 arguments). There is no `==`, ternary, other function, string, or list.
- `prev(x[, n])` is `x` `n` closed candles earlier (default 1, at most 200); `x` may be an expression.
- `percentile(x, n, p)` is the p-th percentile (0–100) of `x` over the `n` candles (at most 500) before the latest.
- `crosses_above(a, b)` / `crosses_below(a, b)` is a crossing at the latest candle: `prev(a) <= prev(b) && a > b`, and the reverse.
- `of("BTCUSDT", x)` is `x` of another coin, which must be in the administrator's favorites; `of` cannot contain `of`.
- Limits: 2000 characters and 20 comparisons (a crossing counts once).
- Everything is calculated on closed candles. MAX and MIN include the latest candle, so a breakout of the previous range is `h_close > prev(h_max_12_high, 1)`. A division by zero is unknown, so it raises no alert.
- A strategy is evaluated on every coin in the administrator's favorites; an alert fires when the expression becomes true.
