import type {
	CandlestickData,
	UTCTimestamp,
	WhitespaceData,
} from "lightweight-charts";
import type { CandleInterval } from "@/api/generated/models";
import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

const dateTimeFormatter = new Intl.DateTimeFormat("en", {
	day: "numeric",
	hour: "2-digit",
	hourCycle: "h23",
	minute: "2-digit",
	month: "short",
	timeZone: "UTC",
});

export type ChartCandle = CandlestickData<UTCTimestamp>;
export type ChartCandleSlot = ChartCandle | WhitespaceData<UTCTimestamp>;

export function toUtcTimestamp(value: string): UTCTimestamp {
	return (Date.parse(value) / 1_000) as UTCTimestamp;
}

export function createCandlestickData(
	candles: readonly PriceCandle[],
	interval: CandleInterval,
): ChartCandleSlot[] {
	const data: ChartCandleSlot[] = [];
	for (let index = 0; index < candles.length; index++) {
		const candle = candles[index];
		if (!candle) continue;
		if (index > 0) {
			let missing = nextCandleOpen(
				candles[index - 1]?.open_time ?? "",
				interval,
			);
			for (
				let count = 0;
				Date.parse(missing) < Date.parse(candle.open_time) && count < 500;
				count++
			) {
				data.push({ time: toUtcTimestamp(missing) });
				missing = nextCandleOpen(missing, interval);
			}
		}
		data.push({
			close: candle.close,
			high: candle.high,
			low: candle.low,
			open: candle.open,
			time: toUtcTimestamp(candle.open_time),
		});
	}
	return data;
}

export function nextCandleOpen(
	value: string,
	interval: CandleInterval,
): string {
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

export function formatPrice(value: number): string {
	return formatNumber(value);
}

export function chartPriceResolution(data: readonly ChartCandleSlot[]): {
	base: number;
	minMove: number;
} {
	const prices = data.flatMap((item) =>
		isChartCandle(item) ? [item.open, item.high, item.low, item.close] : [],
	);
	const smallest = Math.min(...prices.filter((price) => price > 0));
	// Keep eight significant digits (at least cents), bounded so both powers
	// remain finite and nonzero even for subnormal prices. Pass base explicitly:
	// taking the reciprocal of minMove can produce an invalid tick-calculator base.
	const exponent = Number.isFinite(smallest)
		? Math.min(308, Math.max(2, 7 - Math.floor(Math.log10(smallest))))
		: 2;
	return { base: 10 ** exponent, minMove: 10 ** -exponent };
}

export function formatUtcTimestamp(
	value: string | number,
	interval: CandleInterval = "1h",
): string {
	const timestamp =
		typeof value === "number" ? value * 1_000 : Date.parse(value);
	const date = new Date(timestamp);
	if (interval === "1h") return `${dateTimeFormatter.format(date)} UTC`;
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

export function formatCandleRange(candle: PriceCandle | ChartCandle): string {
	return formatRangePercent(((candle.high - candle.low) / candle.open) * 100);
}

export function formatOhlc(candle: PriceCandle | ChartCandle): string {
	return `O ${formatPrice(candle.open)}  H ${formatPrice(candle.high)}  L ${formatPrice(candle.low)}  C ${formatPrice(candle.close)}`;
}

export function isChartCandle(value: unknown): value is ChartCandle {
	return (
		typeof value === "object" &&
		value !== null &&
		"open" in value &&
		"high" in value &&
		"low" in value &&
		"close" in value
	);
}
