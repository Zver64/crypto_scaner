---
name: backtest
description: Create and backtest Crypto Scanner strategies (CEL expressions) against stored candle history through the `scanner` CLI. Use when asked to test, backtest, compare, or tune a scanner strategy or buy/sell warning signal on local or production data.
---

# Backtesting strategies with `scanner`

## Read first

- `backend/cmd/scanner/README.md`: the commands, how backtests trade and judge signals, and what they print.
- `docs/strategy-language.md`: the expression language, variable names, limits, signals, and exits.
- `docs/buy-strategies.md`: the existing strategies.
- `scanner help <command>` for the exact flags; never guess them.

## Rules

- The user installs the CLI (`make install-cli`) and logs in with an API token from the Mini App. Never ask for, read, or print the token or `~/.config/scanner/config.json`. Add `--profile prod` for production.
- Create a signal (`scanner signals`) when the user asks for an indicator, warning, or buy/sell signal; a bearish warning such as “a drop may follow” is `--direction short`. Pick the `--window` that matches the horizon the signal claims. Write exits, take profit, and stop loss only for explicitly requested trading; for a trading bot with a range, the take profit is its upper and the stop loss its lower bound.
- A coin read through `of` must be in `scanner favorites`; otherwise ask the user to add it, never work around it.
- Never pass `--add-indicators` unless the user asks. Only the user enables strategies, in the Mini App.
- Backtests ignore the market cap range, so backtest any coin regardless of it.
- If the evaluated period is too short (about 2,000 candles, roughly 83 days of 1h), ask the user to load deeper history from the Mini App's Commands section; the CLI cannot.
- Backtests run one at a time per machine; never start them in parallel.

## Workflow

1. Write the rules; `scanner vars --filter <text>` lists the variables, `scanner validate` checks an expression.
2. Backtest them unsaved with `scanner strategies backtest` or `scanner signals backtest` on several coins, one after another. `--json` shows the values each rule read at its signals; use them to check that signals fire on the pattern you meant.
3. Save only what works with `create` (always disabled), and tell the user what you saved so they can delete what does not work. `scanner backtest --strategy <id>` backtests a saved one.

## Judging results

Keep parameters universal: never tune thresholds per coin or fit them to one period, `pnl` and `bars_held` exits included. Reject ideas that need a visual chart check; strategies must be algorithmic. Develop on one part of the history (`--to`) and confirm on the rest (`--from`) without changing anything in between. One coin, one event, or one lucky trade proves nothing.

Trading strategies:

- Judge the edge over the baselines, not the net profit alone: it must beat `Buy & Hold`, and for accumulating strategies `DCA`; otherwise holding or buying blindly was better, which is common in uptrends. Check the max drawdown too.
- A short strategy has no baselines and its backtest ignores funding, leverage, and liquidation: judge it by its own net profit, drawdown, and trade statistics, and say that these costs are left out.
- Require a profit factor above 1 and a win rate that holds across coins, over enough closed trades (a few dozen per coin); a single open trade is not a result.
- For bot ranges, the share of take profit against stop loss exits and the candles held tell whether entry and range fit together.

Signals:

- Judge by how far the `Successful` share of `Signals` exceeds that of `All candles`, with the median move to target as support, over enough counted signals per coin, consistently across coins and across `--to`/`--from` halves.
- Respect an explicitly requested recent-event scope: do not require deeper history, and do not claim general predictive reliability from that event alone.
