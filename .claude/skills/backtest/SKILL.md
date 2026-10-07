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
2. Write the expression per `docs/strategy-language.md` (names, `prev`, `percentile`, `of`, limits); `docs/buy-strategies.md` has the existing strategies. `scanner vars --filter rsi` lists configured variables.
3. `scanner validate '<expr>'` until it prints `ok`.
4. `scanner strategies create '<name>' --expr '<expr>'` saves it disabled, adding the indicators it reads that are not configured; it prints the new id. `scanner strategies` lists every saved strategy with its id. `scanner strategies update <id> [--expr …] [--name …] [--message …]` edits only a disabled strategy and refuses an enabled one (edit it in the Mini App). Only the user enables strategies, in the Mini App.
5. `scanner backtest --strategy <id> --symbol BTCUSDT [--hold N]` on any active coin, one coin per run. It simulates trades one position at a time: entry at the open after an alert, exit at the close of the `hold`-th candle of the interval (default `1h` 24, `1d` 7, `1w` 4, `1M` 3; `--hold` 1 to 1000), 0.1% fee on each side. Alerts during an open position are skipped; trades the stored consecutive candles cannot close (end of history or a gap) are unfinished and left out. It prints a header table (coin, period and interval, hold, fee, skipped alerts, unfinished), the strategy metrics each `Compared with` its baseline (net profit with `Buy & Hold` over the same period; trades, win rate, and profit factor with `Every candle`, every evaluated candle taken as an entry with the same hold and fees, overlapping), the average returns against `Every candle`, and the newest 50 trades, newest first; without closed trades, only the header and a `No trades: …` reason. `--json` prints the raw response with every trade and the equity curve.
6. Backtests use only the history stored on the server and never load more. If the evaluated period is too short (about 2,000 candles, roughly 83 days of 1h), ask the user to load deeper history for those coins and that interval from the admin Commands section of the Mini App, then backtest again.
7. Judge a strategy by its edge over the baselines, not by its own net profit:
   - Average trade % and win rate must beat `Every candle` (what entering anywhere with the same hold gives), and a profit factor above 1 should beat its profit factor too.
   - Net profit must beat `Buy & Hold`; otherwise holding the coin was better, which is common in uptrends. Check the max drawdown as well.
   - Only with enough closed trades (a few dozen per coin; a handful proves nothing) and consistently across several coins, backtested one after another (never in parallel). One coin or one lucky trade proves nothing.
   - Keep the default hold unless the strategy's idea needs another; never fit the hold to one coin or period.

Keep parameters universal: never tune thresholds per coin or fit them to one period. Reject ideas that need a visual chart check; strategies must be algorithmic. Tell the user which strategies you created, so they can delete the ones that do not work.
