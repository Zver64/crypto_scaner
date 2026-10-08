import type { LineData, UTCTimestamp } from "lightweight-charts";
import type { BacktestEquityPoint } from "@/api/generated/models";
import { toUtcTimestamp } from "@/features/candle-chart/utils";
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
