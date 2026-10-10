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

- The user installs the CLI (`make install-cli`) and logs in with an API token from the Mini App. Never ask for, read, or print the token or `~/.config/scanner/config.json`. Select an existing profile for the intended server with `--profile <name>`; `scanner profiles` lists names and server URLs without tokens. Never assume the production profile is named `prod`.
- Create a signal (`scanner signals`) when the user asks for an indicator, warning, or buy/sell signal; a bearish warning such as “a drop may follow” is `--direction short`. Pick the `--window` that matches the horizon the signal claims. Write exits, take profit, and stop loss only for explicitly requested trading; for a long trading bot with a range, the take profit is its upper and the stop loss its lower bound; for short, the take profit is its lower and the stop loss its upper bound. A short trading strategy requires both take profit and stop loss.
- A coin read through `of` must be in `scanner favorites`; otherwise ask the user to add it, never work around it.
- Never pass `--add-indicators` unless the user asks. Only the user enables strategies, in the Mini App.
- Backtests ignore the market cap range and run on any active coin, not only favorites. Only coins read through `of` must be active administrator favorites.
- Judge history sufficiency by the actual evaluated period and the number of evaluated or counted events, not just the requested dates. Initial synchronization provides about 2,000 stored candles (roughly 83 days of 1h); indicator warm-up can shorten the evaluated period. Backtests can use up to 20,000 stored candles per interval. If history is insufficient for the requested analysis, ask the user to load deeper history from the Mini App's Commands section; the CLI cannot. Do not require deeper history for an explicitly requested recent-event analysis.
- Backtests run one at a time per machine; never start them in parallel.
- Monitor server execution time after every backtest (`Execution time` in text output, `duration_ms` in JSON). Compare runs on the same server over comparable stored history and periods, using the same client and similar server load. The CLI does not request chart snapshots; the Mini App does, and their preparation is included in its server execution time. A sharp increase in execution time as the formula grows is a sign that it needs simplification: remove redundant conditions and unnecessarily expensive nested calculations while preserving the intended signal or trading logic, then backtest again to verify both the results and the runtime.

## Workflow

1. Write the rules; `scanner vars --filter <text>` lists the variables, `scanner validate` checks an expression.
2. Backtest them unsaved with `scanner strategies backtest` or `scanner signals backtest` on several coins, one after another. Unconfigured indicators can be read without adding them to the Mini App. `--json` returns all rows and the diagnostic values each rule read at its signals; use them to check that signals fire on the pattern you meant. Text output shows at most 100 latest trades or signals. For signal backtests, `--indicators` adds current outputs of local indicator dependencies to the text table; these are distinct from diagnostic expression reads.
3. Save only what works with `create` (always disabled), and tell the user what you saved so they can delete what does not work. `scanner backtest --strategy <id>` backtests either a saved strategy or a saved signal.

## Judging results

Keep parameters universal: never tune thresholds per coin or fit them to one period, `pnl` and `bars_held` exits included. Reject ideas that need a visual chart check; strategies must be algorithmic. Develop on one part of the history (`--to`) and confirm on the rest (`--from`) without changing anything in between. One coin, one event, or one lucky trade proves nothing.

Trading strategies:

- For long strategies, judge the edge over the available baselines, not the net profit alone: it must beat `Buy & Hold`, and for accumulating strategies `DCA`; otherwise holding or buying blindly was better, which is common in uptrends. Check the max drawdown too.
- A short strategy has no baselines and its backtest ignores funding, leverage, and liquidation: judge it by its own net profit, drawdown, and trade statistics, and say that these costs are left out.
- Require a profit factor above 1 when it is defined, and a win rate that holds across coins, over enough closed trades (a few dozen per coin). Profit factor is `null` (shown as `-` in text) when there are no losing trades; distinguish an all-winning sample from a lack of closed trades rather than rejecting either solely because the metric is absent. An open trade contributes to net profit and drawdown but not closed-trade statistics; a single open trade is not a result.
- For bot ranges, the share of take profit against stop loss exits and the candles held tell whether entry and range fit together.

Signals:

- `--window` counts candles after each signal, not hours; `--target-ratio` sets the target distance in ATR(14) stops. Interpret success according to direction: long/short must reach the target before the stop; sideways must avoid both targets throughout the window. Unevaluable signals are not failures.
- `--to` limits which signal candles are selected, but their outcomes may read stored candles after that bound. For independent development and validation samples, leave at least one evaluation window between the last development candle and the first validation candle; date flags include the whole UTC day. Do not treat adjacent `--to`/`--from` periods as outcome-independent.

- Judge by how far the `Successful` share of `Signals` exceeds that of `All candles`, with the median move to target as support, over enough counted signals per coin, consistently across coins and across `--to`/`--from` halves.
- Respect an explicitly requested recent-event scope: do not require deeper history, and do not claim general predictive reliability from that event alone.
