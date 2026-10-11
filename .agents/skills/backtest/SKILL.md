---
name: backtest
description: Create and backtest Crypto Scanner strategies (CEL expressions) against stored candle history through the `scanner` CLI. Use when asked to test, backtest, compare, or tune a scanner strategy or buy/sell warning signal on local or production data.
---

# Backtesting strategies with `scanner`

## Read first

- `backend/cmd/scanner/README.md`: commands, backtest execution, signal evaluation, and output.
- `docs/strategy-language.md`: expressions, variables, limits, signals, and exits.
- `docs/buy-strategies.md`: existing strategies.
- `scanner help <command>`: exact flags; never guess them.

## Rules

- The user installs the CLI (`make install-cli`) and logs in with a Mini App API token. Never ask for, read, or print the token or `~/.config/scanner/config.json`. Use `scanner profiles` (names and server URLs, no tokens) to select the intended server with `--profile <name>`; never assume production is named `prod`.
- Use `scanner signals` for an indicator, warning, or buy/sell signal; a bearish warning is `--direction short`. Match `--window` to the claimed horizon. Add exits, take profit, and stop loss only for explicitly requested trading. For a long bot range, take profit is the upper bound and stop loss the lower; for short, reverse them. Short strategies require both.
- Backtests ignore market cap ranges and accept any active coin, not only favorites. Coins read through `of` must be active administrator favorites (`scanner favorites`); otherwise ask the user to add them, never work around it.
- Never pass `--add-indicators` unless asked. Only the user enables strategies, in the Mini App.
- Work with stored history. If the user explicitly selects a period or history segment, apply no minimum candle count and do not request a larger sample; report results without generalizing beyond that scope. Otherwise, 2,000 or more stored candles on the signal's interval are sufficient for diagnosis and improvement: do not call this history insufficient or ask for more. Below 2,000, assess how substantial the shortfall is: flag very short history immediately and explain that reliable evaluation needs more; a modest shortfall need not prompt a warning or a request for more history. Continue diagnosis with what is available. Report the actual evaluated period and sample size, distinguishing stored history from candles excluded by indicator warm-up or selected date bounds. Backtests support up to 20,000 stored candles per interval.
- Never run backtests in parallel: only one per machine. For multiple coins, use comma-separated or repeated `--symbol` values; the CLI runs them sequentially under one lock. There is no `--favorites` flag. Multi-coin text shows only per-coin summaries (`--indicators` is ignored); `--json` returns a `status`/`items` envelope with full results/errors. Coin-specific failures continue, global failures stop the queue; any failure exits with 1, so inspect successful results even then.
- Check server execution time after every backtest (`Execution time` / JSON `duration_ms`). Compare on the same server, client, comparable history/periods, and similar load. Mini App timings include chart snapshot preparation; CLI timings do not. If formula growth sharply increases runtime, remove redundant conditions and unnecessarily expensive nested calculations without changing the intended logic; rerun to verify results and runtime.

## Workflow

1. Write the rules using `scanner vars --filter <text>` and check expressions with `scanner validate`.
2. Test unsaved rules on several coins with `scanner strategies backtest` or `scanner signals backtest`, e.g. `scanner signals backtest --expr 'h_rsi > 80' --direction short --symbol BTCUSDT,ETHUSDT,ARBUSDT`. Unconfigured indicators are readable without adding them to the Mini App. `--json` includes all rows and diagnostic expression values at signals; use them to verify the intended pattern. Text shows at most 100 latest trades/signals; for signals, `--indicators` adds current local indicator dependency outputs, distinct from diagnostic reads.
3. Iterate using the diagnosis rules below until the signal meets the user's requirements. Report failure only after at least 50 distinct, backtested and analyzed signal variants; repeated runs of one formula do not count. Keep a brief record of each variant, result, diagnosis, and next change. Do not invent trials or count requests that failed to execute; report execution blockers separately from failure to find a working signal.
4. Save only what works with `create` (always disabled); tell the user what you saved so they can delete it if needed. Use `scanner backtest --strategy <id>` for saved strategies or signals.

## Diagnosing signals

- Analyze every backtest before choosing the next change; aggregate success rates alone are not a diagnosis. Rerun individual coins when more detailed diagnostics are needed. For false or missing signals, retrieve stored candles around the actual or expected occurrence through supported read-only interfaces (verify available commands/endpoints; never invent them). Inspect diagnostic values, pivot chronology, and preceding structure, especially the trend and first versus repeated structural break; identify why the formula fired or failed and correct that cause.
- On available history of 2,000 or more candles outside a user-selected segment, zero or only 1–2 signals indicate a problem in the agent's formula, not insufficient history or mere rarity. Investigate overcomplication, incorrect encoding, and filters suppressing valid setups; a low count is not an excuse to stop.
- Keep every variant faithful to the requested setup. Never merely loosen thresholds to manufacture events or substitute an earlier or different signal to increase the count.

## Judging results

Keep parameters universal: never tune thresholds per coin or fit one period, including `pnl` and `bars_held` exits. Reject ideas requiring a visual chart check; strategies must be algorithmic. Develop with `--to`, then validate with `--from` without changing rules between samples. One coin, event, or lucky trade proves nothing. Respect an explicitly requested recent-event scope without claiming general predictive reliability from it.

### Trading strategies

- Long: require an edge over `Buy & Hold`, and over `DCA` for accumulating strategies; net profit alone is insufficient, especially in uptrends. Check max drawdown.
- Short: judge net profit, drawdown, and trade statistics; there are no baselines. Disclose that funding, leverage, and liquidation are ignored.
- Require profit factor above 1 when defined and a win rate that holds across coins with enough closed trades (a few dozen per coin). Profit factor is `null` (`-` in text) without losing trades: distinguish all-winning samples from no closed trades; do not reject solely because it is absent. Open trades affect net profit/drawdown, not closed-trade statistics; a single open trade is not a result.
- For bot ranges, compare take profit versus stop loss exits and candles held to assess entry/range fit.

### Signals

- `--window` counts candles after the signal, not hours; `--target-ratio` sets target distance in ATR(14) stops. Long/short must reach the target before the stop; sideways must avoid both targets throughout the window. Unevaluable signals are not failures.
- `--to` limits signal candles, not outcome candles, which may follow the bound. For independent samples, leave at least one evaluation window between the last development candle and first validation candle; date flags include the whole UTC day. Adjacent `--to`/`--from` periods are not outcome-independent.
- Judge how far the `Successful` share of `Signals` exceeds `All candles`, supported by median move to target, over enough counted signals per coin and consistently across coins and development/validation samples.
