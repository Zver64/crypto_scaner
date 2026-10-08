import type { LineData, UTCTimestamp } from "lightweight-charts";
import {
	type BacktestEquityPoint,
	type BacktestSignalWindow,
	type CandleInterval,
	Direction,
	type StrategyBacktest,
} from "@/api/generated/models";
import type { ChartTradeMarkers } from "@/features/candle-chart/types";
import { toUtcTimestamp } from "@/features/candle-chart/utils";
import {
	hitsColorLightness,
	hitsColorSaturation,
} from "@/features/strategy-backtest/config";
import {
	intervalUnits,
	longTradeWords,
	shortTradeWords,
} from "@/features/strategy-backtest/constants";
import type {
	SignalWindowRow,
	TradeWords,
} from "@/features/strategy-backtest/types";
import { shiftedUtcTime } from "@/utils/date-time-format";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

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
// or the signals of a signal, under their candles for a rise, over them for
// a fall, and as circles for sideways. A long trade opens on a rise mark and
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
		return {
			entries: direction === Direction.long ? times : [],
			exits: direction === Direction.short ? times : [],
			marks: direction === Direction.sideways ? times : [],
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

// Two rows per window: the moves after the signals, then after every
// candle.
export function signalWindowRows(
	windows: readonly BacktestSignalWindow[],
): SignalWindowRow[] {
	return windows.flatMap(({ all, candles, signals }) => [
		{
			candles,
			key: `${candles}-signals`,
			label: "Signals",
			signals: true,
			stats: signals,
		},
		{
			candles: undefined,
			key: `${candles}-all`,
			label: "All candles",
			signals: false,
			stats: all,
		},
	]);
}

// The color of a share of hits, 0 to 1: its hue runs from red at 0 through
// yellow to green at 1.
export function hitsColor(hits: number): string {
	const hue = Math.min(Math.max(hits, 0), 1) * 120;
	return `hsl(${hue} ${hitsColorSaturation}% ${hitsColorLightness}%)`;
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
