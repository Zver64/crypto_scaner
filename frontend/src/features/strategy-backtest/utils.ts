import type { LineData, UTCTimestamp } from "lightweight-charts";
import {
	type BacktestEquityPoint,
	type BacktestSignal,
	type BacktestSignalStats,
	type CandleInterval,
	Direction,
	type StrategyBacktest,
} from "@/api/generated/models";
import type { ChartTradeMarkers } from "@/features/candle-chart/types";
import { toUtcTimestamp } from "@/features/candle-chart/utils";
import {
	successColorLightness,
	successColorSaturation,
} from "@/features/strategy-backtest/config";
import {
	intervalUnits,
	longTradeWords,
	shortTradeWords,
	signalArrowOptions,
	signalMarkerColors,
} from "@/features/strategy-backtest/constants";
import type {
	SignalSummaryRow,
	TradeWords,
	TradingSummaryRow,
} from "@/features/strategy-backtest/types";
import { shiftedUtcTime } from "@/utils/date-time-format";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

export function tradingSummaryRows(
	backtest: StrategyBacktest,
): TradingSummaryRow[] {
	const stats = backtest.summary.stats;
	const count = (value: number) => formatNumber(value);
	const metric = (label: string, value: string, compared = "—") => ({
		label,
		value,
		compared,
	});
	return [
		metric(
			"Net profit",
			formatFractionPercent(backtest.summary.net_profit),
			backtest.direction === Direction.short
				? "—"
				: `Buy & Hold ${formatFractionPercent(backtest.baselines.buy_and_hold)}, DCA ${formatFractionPercent(backtest.baselines.dca)}`,
		),
		metric(
			"Max drawdown",
			formatFractionPercent(backtest.summary.max_drawdown),
		),
		metric("Closed trades", count(stats.trade_count)),
		metric(
			"Open trades",
			count(backtest.trades.filter((trade) => trade.open).length),
		),
		metric("Skipped signals", count(backtest.skipped_alerts)),
		metric("Win rate", formatFractionPercent(stats.win_rate)),
		metric("Profit factor", formatProfitFactor(stats.profit_factor)),
		metric("Avg trade", formatFractionPercent(stats.average_trade)),
		metric("Avg win", formatFractionPercent(stats.average_win)),
		metric("Avg loss", formatFractionPercent(stats.average_loss)),
		metric("Avg candles held", formatProfitFactor(stats.average_bars)),
		metric(
			"Exits TP / SL / rule",
			`${count(stats.take_profit_exits)} / ${count(stats.stop_loss_exits)} / ${count(stats.exit_rule_exits)}`,
		),
	];
}

// Backtest returns are fractions, 0.05 for 5%.
export function fractionToPercent(value: number | null): number | null {
	return value === null ? null : value * 100;
}

export function formatFractionPercent(value: number | null): string {
	const percent = fractionToPercent(value);
	return percent === null ? "—" : formatRangePercent(percent, 2);
}

// A profit factor, or a dash without losses.
export function formatProfitFactor(value: number | null): string {
	return value === null ? "—" : formatNumber(value, 2);
}

// The take profit and stop loss of a trade, a dash for each it lacks, or
// one dash without both.
export function formatPriceLevels(
	takeProfit: number | null,
	stopLoss: number | null,
): string {
	if (takeProfit === null && stopLoss === null) {
		return "—";
	}
	const level = (value: number | null) =>
		value === null ? "—" : formatNumber(value);
	return `${level(takeProfit)} / ${level(stopLoss)}`;
}

// Net profit in percent after each trade, starting from 0 at the first
// evaluated candle, which opens before any exit candle.
export function createEquityData(
	from: string,
	equity: readonly BacktestEquityPoint[],
): LineData<UTCTimestamp>[] {
	return [
		{ time: toUtcTimestamp(from), value: 0 },
		...equity.map(({ equity, time }) => ({
			time: toUtcTimestamp(time),
			value: (equity - 1) * 100,
		})),
	];
}

// The chart marks of a backtest: the first fills and the closes of trades,
// or blue signal entries and outcome-colored closes, with circles for
// sideways. A long trade opens on a rise mark and
// closes on a fall mark, a short trade the other way around.
export function backtestMarkers({
	direction,
	signal,
	trades,
}: Pick<
	StrategyBacktest,
	"direction" | "signal" | "trades"
>): ChartTradeMarkers {
	if (signal) {
		const times = signal.occurrences.map(({ time }) => time);
		if (direction === Direction.sideways) {
			return { entries: [], exits: [], marks: times };
		}
		const short = direction === Direction.short;
		return {
			entries: [],
			exits: [],
			groups: [
				{
					times,
					options: {
						...signalArrowOptions[short ? "down" : "up"],
						color: signalMarkerColors.entry,
					},
				},
				...(["success", "failure", "neutral"] as const).map((outcome) => ({
					times: signal.occurrences.flatMap((occurrence) =>
						occurrence.outcome === outcome && occurrence.exit_time !== null
							? [occurrence.exit_time]
							: [],
					),
					options: {
						...signalArrowOptions[short ? "up" : "down"],
						color: signalMarkerColors[outcome],
					},
				})),
			],
		};
	}
	const opens = trades.map(({ entry_time }) => entry_time);
	// An open trade has not closed.
	const closes = trades
		.filter(({ open }) => !open)
		.map(({ exit_time }) => exit_time);
	return direction === Direction.short
		? { entries: closes, exits: opens }
		: { entries: opens, exits: closes };
}

// What a strategy trading in direction does in its trades.
export function tradeWords(direction: Direction): TradeWords {
	return direction === Direction.short ? shortTradeWords : longTradeWords;
}

// The counted signals beside every candle of a signal's backtest; the
// signals row also counts every evaluated signal.
export function signalSummaryRows(signal: BacktestSignal): SignalSummaryRow[] {
	return [
		{
			count: `${formatNumber(signal.evaluated)} / ${formatNumber(signal.signals.count)}`,
			key: "signals",
			label: "Signals",
			stats: signal.signals,
		},
		{
			count: formatNumber(signal.all.count),
			key: "all",
			label: "All candles",
			stats: signal.all,
		},
	];
}

// The color of a success share, 0 to 1: its hue runs from red at 0 through
// yellow to green at 1.
export function successColor(share: number): string {
	const hue = Math.min(Math.max(share, 0), 1) * 120;
	return `hsl(${hue} ${successColorSaturation}% ${successColorLightness}%)`;
}

// Show successes over decisive outcomes; the backend supplies the rate.
export function formatSignalSuccesses(stats: BacktestSignalStats): string {
	const { count, successes, neutrals } = stats;
	const rate = stats.success_rate;
	if (rate === null) return "—";
	return `${formatNumber(successes)}/${formatNumber(count - neutrals)} (${formatFractionPercent(rate)})`;
}

// How a signal's backtest judges its signals, by its direction, window, and
// target ratio.
export function signalExplanation(
	direction: Direction,
	window: number,
	targetRatio: number,
): string {
	const candles = formatNumber(window);
	const ratio = formatNumber(targetRatio);
	return direction === Direction.sideways
		? `Targets: ${ratio} times ATR(14) on both sides. Window: ${candles} candles. Success: the price touches neither target. Move to target: how far the price moved away from the signal price in either direction. The Signals median covers every counted signal, successful or not.`
		: `Stop: one ATR(14) against the signal; target: ${ratio} times farther in the signal's direction. Window: ${candles} candles. Success: target before stop, or, without either touch, a final close at least one ATR(14) in the signal's direction. Neutral: without either touch, a final close from entry to less than one ATR(14) in that direction; neutrals are excluded from the success percentage. Otherwise failure. Move to target: how far the price went in the signal's direction, also after the target, until the stop is hit or the window ends. The Signals median covers every counted signal, successful or not.`;
}

// The period of a backtest request from its first and last UTC days, the
// whole of each day; an absent day leaves that side open.
export function backtestPeriod(
	from: string | undefined,
	to: string | undefined,
): { from?: string; to?: string } {
	return {
		...(from ? { from: `${from}T00:00:00Z` } : {}),
		...(to ? { to: `${to}T23:59:59Z` } : {}),
	};
}

// Why a backtest evaluated no candle: none stored in the requested period,
// or none stored yet at all.
export function noCandlesMessage(period: boolean): string {
	return period
		? "No stored candles of this interval in the chosen period."
		: "No stored candles of this interval yet; synchronization fills them first.";
}

// Which sides of the chosen period, first and last UTC days as YYYY-MM-DD,
// the evaluated candles of interval fall short of: a candle opening within
// the chosen period before the first evaluated one, or a candle closed by now
// opening within it after the last evaluated one.
export function shortenedPeriod(
	chosen: { from: string | undefined; to: string | undefined },
	evaluated: { from: string; interval: CandleInterval; to: string },
	now: number,
): { end: boolean; start: boolean } {
	const unit = intervalUnits[evaluated.interval];
	return {
		end:
			chosen.to !== undefined &&
			shiftedUtcTime(evaluated.to, 1, unit) <
				shiftedUtcTime(chosen.to, 1, "day") &&
			shiftedUtcTime(evaluated.to, 2, unit) <= now,
		start:
			chosen.from !== undefined &&
			shiftedUtcTime(evaluated.from, -1, unit) >=
				shiftedUtcTime(chosen.from, 0, "day"),
	};
}
