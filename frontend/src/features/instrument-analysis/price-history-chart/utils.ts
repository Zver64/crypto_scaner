import type {
	CandlestickData,
	UTCTimestamp,
	WhitespaceData,
} from "lightweight-charts";
import type { PriceCandle } from "@/api/client";
import { formatNumber } from "@/utils/number-format";

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
	candles: readonly (PriceCandle | null)[],
	windowFrom: string,
): ChartCandleSlot[] {
	const from = Date.parse(windowFrom);
	return candles.map((candle, index) => {
		const time = ((from + index * 3_600_000) / 1_000) as UTCTimestamp;
		return candle === null
			? { time }
			: {
					close: candle.close,
					high: candle.high,
					low: candle.low,
					open: candle.open,
					time,
				};
	});
}

export function availableCandles(
	candles: readonly (PriceCandle | null)[],
): PriceCandle[] {
	return candles.filter((candle): candle is PriceCandle => candle !== null);
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

export function formatUtcTimestamp(value: string | number): string {
	const timestamp =
		typeof value === "number" ? value * 1_000 : Date.parse(value);
	return `${dateTimeFormatter.format(new Date(timestamp))} UTC`;
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
