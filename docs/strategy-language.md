# Strategy language

Reference for writing a strategy expression that the scanner imports unchanged.

## Indicators and variable names

- Indicators are TA-Lib functions. Types, parameter order, defaults, and output names are in `backend/internal/indicator/talib/functions_gen.go` (`indicatorType`, `params`, `outputs`). TA-Lib defaults are not obvious: `ema`, `sma`, `max`, and `min` use 30, `roc` 10, `bbands` 5.
- An indicator title is `<interval>-<type>[-parameters]`. Intervals: `h` (1h), `d` (1d), `w` (1w), `m` (1M). Numeric parameters follow in order up to the last non-default one; trailing defaults are dropped: `h-rsi` is RSI 14, `h-rsi-21`, `h-bbands-20-2.5` (upper band), `h-bbands-20-2-2.5` (lower band). The MA type is a number (`0` SMA, `1` EMA, …).
- The candle field (`source`) is spelled as a word and only when it is not `close`: `h-max-12-high`, `h-sma-20-volume`. Fields: `open`, `high`, `low`, `close`, `volume`, `quote_asset_volume`, `trade_count`.
- A variable is the title with `.` → `p` and other characters → `_`: `h-bbands-20-2.5` → `h_bbands_20_2p5`. An indicator with several outputs appends `_<output>`: `h_bbands_20_2p5_upperband`, `h_macd_macdsignal`, `h_stoch_slowk`.
- Candle fields need no indicator: `<interval>_open`, `_high`, `_low`, `_close`, `_volume`, `_quote_volume`, `_trades` (`h_close`).
- A name must spell back exactly one indicator. Strategies read indicators that are not configured without adding them; the Mini App adds them only before showing the rules in the builder. Do not invent names or use `__`.

## Expressions

- The result is a boolean: comparisons (`<`, `<=`, `>`, `>=`) of numbers and variables combined with `&&`, `||`, and `!`. Arithmetic: `+ - * /`, `abs(x)`, `mod(a, b)`, `min(…)`, `max(…)` (2–10 arguments). There is no `==`, ternary, other function, string, or list.
- `prev(x[, n])` is `x` `n` closed candles earlier (default 1, at most 200); `x` may be an expression.
- `percentile(x, n, p)` is the p-th percentile (0–100) of `x` over the `n` candles (at most 500) before the latest.
- `crosses_above(a, b)` / `crosses_below(a, b)` is a crossing at the latest candle: `prev(a) <= prev(b) && a > b`, and the reverse.
- `of("BTCUSDT", x)` is `x` of another coin, which must be in the administrator's favorites; `of` cannot contain `of`.
- Limits: 2000 characters and 20 comparisons (a crossing counts once).
- Everything is calculated on closed candles. MAX and MIN include the latest candle, so a breakout of the previous range is `h_close > prev(h_max_12_high, 1)`. A division by zero is unknown, so it signals nothing.

## Entry, exit, and trades

- A strategy has an entry rule and, optionally, an exit rule, a take profit, and a stop loss. The rules are conditions of this language; the take profit and the stop loss are prices (see below). It trades every coin in the administrator's favorites on the closed candles of the finest interval its expressions read.
- An entry signal is the entry rule turning from false to true at a candle close. A strategy that exits, through an exit rule, a take profit, or a stop loss, holds one buy per trade: a signal opens a trade, and signals during the trade buy nothing. A strategy without any exit never sells: every entry signal buys.
- The take profit and the stop loss are price expressions evaluated at the close of the signal and fixed for the trade. A signal whose take profit is not above its close, or whose stop loss is not between 0 and its close, buys nothing. A candle that reaches the stop loss (its low at or below it) or the take profit (its high at or above it) sells the trade at that price, or at the candle's open when it opens past it; a candle that reaches both sells at the stop loss. This happens on the candle the buy fills at too.
- A strategy may limit the coins it buys to a range of current market caps in USD, from CoinGecko; either bound may be absent. An entry signal on a coin outside the range, or of an unknown market cap while any bound is set, buys nothing, but the entry still counts as true, so the coin buys only after its entry turns true again within the range. Open trades sell as usual. Backtests ignore the range and trade any coin.
- An exit signal is the exit rule being true at a candle close after the first buy. It sells every buy of the trade at the next open. The take profit and stop loss come first on their candle, and an exit wins over an entry on the same candle; after any sell the entry counts as false on that candle, so an entry still true on the next candle signals again.
- Signals are decided at the close and buys fill at the next candle's open; every buy spends the same amount. A candle is decided only once every expression it needs has its values. Live alerts check only the latest closed candle, so candles that close while the backend is down are skipped; a take profit or stop loss alert arrives after the close of the candle that reached it.
- Only the exit rule reads the position variables of the open trade, at the latest candle only (not inside `prev`, `percentile`, crossings, or `of`): `entry_price`, the average price of its buys; `pnl`, its return at the close before fees in percent (`5` is 5%); and `bars_held`, the candles since its first buy filled, counting that candle. An exit rule may read only them: `pnl >= 5 || pnl <= -3 || bars_held >= 24`.
- A price expression is arithmetic over variables and numbers with the functions above, `prev`, `percentile`, and `of`, without comparisons, that reads the evaluated coin: `h_close * 1.05`, `h_bbands_20_2_2_upperband`, `h_close - 2 * h_atr_14`, `min(h_low, prev(h_low))`. It cannot read the position variables.
- Telegram alerts announce every buy, with the take profit and stop loss of the trade, and every sell, with what sold it. Enabling or changing how a strategy trades starts it afresh, without trades; an entry true at that moment buys only once it turns true again.
