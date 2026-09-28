import type {
	ChartIndicatorOptions,
	ChartReadoutOptions,
} from "@/components/price-history-chart";
import { formatRangePercent } from "@/utils/range-percent";

export const rsiIndicator: ChartIndicatorOptions = {
	bounds: { min: 0, max: 100 },
	lines: [
		{ price: 30, title: "RSI 30" },
		{ price: 70, title: "RSI 70" },
	],
	formatValue: (value) => value.toFixed(1),
	minMove: 0.1,
};

export const rangeReadout: ChartReadoutOptions = {
	label: "R",
	format: (candle) =>
		formatRangePercent(((candle.high - candle.low) / candle.open) * 100),
};
