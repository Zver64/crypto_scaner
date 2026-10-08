---
name: backtest
description: Create and backtest Crypto Scanner strategies (CEL expressions) against stored candle history through the `scanner` CLI. Use when asked to test, backtest, compare, or tune a scanner strategy or buy signal on local or production data.
---

# Backtesting strategies with `scanner`

## Prerequisites

- The user installs the CLI with `make install-cli` and logs in with `scanner login dev --server http://localhost:8080`, pasting an API token from the Mini App settings into stdin. Never ask for, read, or print the token or `~/.config/scanner/config.json`.
- `scanner profiles` shows the servers; add `--profile prod` to any command for production. `scanner help` lists the commands, `scanner help <command>` its flags.

## Workflow

Creating and backtesting are separate steps: only saved strategies are backtested.

1. `scanner favorites`: the coins `of("SYM", x)` can read. A coin read through `of` that is not a favorite has unknown values, so the strategy never fires. Ask the user to add it to favorites; do not work around it.
2. Write the entry rule and the exits per `docs/strategy-language.md` (names, `prev`, `percentile`, `of`, limits, signals, and the exit-only position variables `entry_price`, `pnl`, `bars_held`); `docs/buy-strategies.md` has the existing strategies. `scanner vars --filter rsi` lists configured variables. Strategies mostly start trading bots, which get an entry plus a range: the take profit is its upper and the stop loss its lower bound, both price expressions fixed at the signal (`h_close * 1.05`, `h_close - 2 * h_atr_14`). An exit rule sells on a condition instead. With any exit the strategy holds one buy per trade; without any it only accumulates, buying at every entry signal.
3. `scanner validate '<expr>'` (with `--exit` for an exit rule, `--price` for a take profit or stop loss) until it prints `ok`.
4. `scanner strategies create '<name>' --expr '<expr>' [--exit '<exit>'] [--take-profit '<price>'] [--stop-loss '<price>']` saves it disabled and prints the new id. It reads indicators that are not configured, such as `h_atr_100`, without adding them, so nothing needs cleaning up afterwards; never pass `--add-indicators` unless the user asks. `scanner strategies` lists every saved strategy with its id, rules, and how it buys. `scanner strategies update <id> [--expr …] [--exit …] [--take-profit …] [--stop-loss …] [--name …] [--message …]` edits only a disabled strategy and refuses an enabled one (edit it in the Mini App); an empty value removes an exit. Only the user enables strategies, in the Mini App.
5. `scanner backtest --strategy <id> --symbol BTCUSDT [--from DATE] [--to DATE]` on any active coin, one coin per run. It replays the strategy's own trades exactly as live signals trade: buys and exit rule sells fill at the open after their signal, a take profit or stop loss sells at its price on the candle reaching it (the stop loss first when a candle reaches both), every buy spends the same amount, 0.1% fee on each side, and a trade the history ends before selling stays open, valued at the last close. It prints a header table (coin, period and interval, fee, entry signals that bought nothing), the metrics (net profit compared with `Buy & Hold` and `DCA`, buying the same amount on every candle; max drawdown at every candle close; closed-trade statistics, average candles held, and exits by take profit, stop loss, and exit rule), and the newest 50 trades with their buys, average entry, and what sold them, newest first; without trades, only the header and a `No trades: …` reason. `--json` prints the raw response with the equity curve and every trade: its fills (signal time, fill time and price, and the values the entry rule read at the signal), take profit and stop loss, exit reason, and the values the exit rule read at its signal. Use them to check that signals fire on the pattern you meant.
6. Backtests use only the history stored on the server and never load more. If the evaluated period is too short (about 2,000 candles, roughly 83 days of 1h), ask the user to load deeper history for those coins and that interval from the admin Commands section of the Mini App, then backtest again.
7. Judge a strategy by its edge over the baselines, not by its own net profit:
   - Net profit must beat `Buy & Hold`, and for accumulating strategies `DCA`; otherwise holding or buying blindly was better, which is common in uptrends. Check the max drawdown as well.
   - For bot ranges, the share of trades the take profit closes against the stop loss, and the candles held, tell whether entry and range fit together.
   - Develop on one part of the history (`--to`) and confirm on the rest (`--from`) without changing anything in between.
   - A profit factor above 1 and a win rate that holds across coins; a single open trade is not a result.
   - Only with enough closed trades (a few dozen per coin; a handful proves nothing) and consistently across several coins, backtested one after another (never in parallel). One coin or one lucky trade proves nothing.
   - Never fit exit thresholds such as `pnl` targets or `bars_held` to one coin or period.

Keep parameters universal: never tune thresholds per coin or fit them to one period. Reject ideas that need a visual chart check; strategies must be algorithmic. Tell the user which strategies you created, so they can delete the ones that do not work.
