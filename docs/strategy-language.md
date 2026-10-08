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
- Everything is calculated on closed candles. MAX and MIN include the latest candle, so a breakout of the previous range is `h_close > prev(h_max_12_high, 1)`. A division by zero is unknown, so it signals nothing.

## Entry, exit, and trades

- A strategy has an entry rule and an optional exit rule, both expressions of this language. It trades every coin in the administrator's favorites on the closed candles of the finest interval its rules read.
- An entry signal is the entry rule turning from false to true at a candle close. It buys: it opens a trade, or adds a buy to the open trade when the strategy accumulates or has no exit rule, up to the max buys of a trade (0: no limit). Other entry signals during a trade buy nothing.
- An exit signal is the exit rule being true at a candle close after the first buy. It sells every buy of the trade, and the entry counts as false on that candle, so an entry still true on the next candle signals again. An exit wins over an entry on the same candle.
- Without an exit rule a strategy never sells: every entry signal buys, up to the max buys. Accumulation applies only with an exit rule; without one it is stored as off.
- Signals are decided at the close and filled at the next candle's open; every buy spends the same amount. A candle is decided only once every rule it needs has its values. Live alerts check only the latest closed candle, so candles that close while the backend is down are skipped.
- Only the exit rule reads the position variables of the open trade, at the latest candle only (not inside `prev`, `percentile`, crossings, or `of`): `entry_price`, the average price of its buys; `pnl`, its return at the close before fees in percent (`5` is 5%); and `bars_held`, the candles since its first buy filled, counting that candle. An exit rule may read only them: `pnl >= 5 || pnl <= -3 || bars_held >= 24`.
- Telegram alerts announce every buy and sell. Enabling or changing how a strategy trades starts it afresh, without trades; an entry true at that moment buys only once it turns true again.
