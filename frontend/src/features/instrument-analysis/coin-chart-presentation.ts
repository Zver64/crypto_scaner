import type {
	ChartIndicatorOptions,
	ChartIntervalOption,
	ChartReadoutOptions,
} from "@/components/price-history-chart";
import { formatRangePercent } from "@/utils/range-percent";

export const coinChartIntervals = [
	{ label: "Hourly", value: "1h", showTime: true },
	{ label: "Daily", value: "1d" },
	{ label: "Weekly", value: "1w" },
	{ label: "Monthly", value: "1M" },
] as const satisfies readonly [ChartIntervalOption, ...ChartIntervalOption[]];

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

const hourlyFormatter = new Intl.DateTimeFormat("en", {
	day: "numeric",
	hour: "2-digit",
	hourCycle: "h23",
	minute: "2-digit",
	month: "short",
	timeZone: "UTC",
});

export function formatCoinChartTime(
	value: string | number,
	interval: string,
): string {
	const timestamp =
		typeof value === "number" ? value * 1_000 : Date.parse(value);
	const date = new Date(timestamp);
	if (interval === "1h") return `${hourlyFormatter.format(date)} UTC`;
	if (interval === "1M") {
		return `${new Intl.DateTimeFormat("en", { month: "short", year: "numeric", timeZone: "UTC" }).format(date)} UTC`;
	}
	const day = new Intl.DateTimeFormat("en", {
		day: "numeric",
		month: "short",
		year: "numeric",
		timeZone: "UTC",
	}).format(date);
	return interval === "1w" ? `Week of ${day} UTC` : `${day} UTC`;
}

export function nextCoinCandleOpen(value: string, interval: string): string {
	const date = new Date(value);
	switch (interval) {
		case "1h":
			date.setUTCHours(date.getUTCHours() + 1);
			break;
		case "1d":
			date.setUTCDate(date.getUTCDate() + 1);
			break;
		case "1w":
			date.setUTCDate(date.getUTCDate() + 7);
			break;
		case "1M":
			date.setUTCMonth(date.getUTCMonth() + 1);
			break;
	}
	return date.toISOString();
}
