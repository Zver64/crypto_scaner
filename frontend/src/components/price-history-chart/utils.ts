import type { IRange, UTCTimestamp } from "lightweight-charts";
import type {
	ChartCandle,
	ChartCandleSlot,
	ChartIndicatorSlot,
	ChartInterval,
	ChartVolumeSlot,
	IndicatorPoint,
	PriceCandle,
} from "@/components/price-history-chart/types";
import { formatNumber } from "@/utils/number-format";

export function toUtcTimestamp(value: string): UTCTimestamp {
	return (Date.parse(value) / 1_000) as UTCTimestamp;
}

export function createCandlestickData(
	candles: readonly PriceCandle[],
	interval: ChartInterval,
	nextOpen: (time: string, interval: ChartInterval) => string,
): ChartCandleSlot[] {
	const data: ChartCandleSlot[] = [];
	for (let index = 0; index < candles.length; index++) {
		const candle = candles[index];
		if (!candle) continue;
		if (index > 0) {
			let missing = nextOpen(candles[index - 1]?.open_time ?? "", interval);
			for (
				let count = 0;
				Date.parse(missing) < Date.parse(candle.open_time) && count < 500;
				count++
			) {
				data.push({ time: toUtcTimestamp(missing) });
				missing = nextOpen(missing, interval);
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

export function createIndicatorData(
	data: readonly ChartCandleSlot[],
	points: readonly IndicatorPoint[],
): ChartIndicatorSlot[] {
	const values = new Map(
		points.map((point) => [toUtcTimestamp(point.time), point.value]),
	);
	return data.map(({ time }) => {
		const value = values.get(time);
		return value === undefined ? { time } : { time, value };
	});
}

export function createVolumeData(
	data: readonly ChartCandleSlot[],
	candles: readonly PriceCandle[],
	colors: { down: string; up: string },
): ChartVolumeSlot[] {
	const volumes = new Map(
		candles.map((candle) => [toUtcTimestamp(candle.open_time), candle]),
	);
	return data.map(({ time }) => {
		const candle = volumes.get(time);
		return candle === undefined
			? { time }
			: {
					color: candle.close >= candle.open ? colors.up : colors.down,
					time,
					value: candle.volume,
				};
	});
}

export function getVisibleMinMax(
	data: readonly ChartCandleSlot[],
	range: IRange<number> | null,
): { min: number; max: number } | null {
	if (range === null) return null;
	const from = Math.max(0, Math.floor(range.from));
	const to = Math.min(data.length - 1, Math.ceil(range.to));
	let min = Number.POSITIVE_INFINITY;
	let max = Number.NEGATIVE_INFINITY;
	for (let index = from; index <= to; index++) {
		const candle = data[index];
		if (!candle || !("low" in candle)) continue;
		min = Math.min(min, candle.low);
		max = Math.max(max, candle.high);
	}
	return Number.isFinite(min) && Number.isFinite(max) ? { min, max } : null;
}

export function formatPrice(
	value: number,
	maximumFractionDigits?: number,
): string {
	return formatNumber(value, maximumFractionDigits);
}

export function formatOhlc(
	candle: Pick<PriceCandle, "open" | "high" | "low" | "close">,
): string {
	return `O ${formatPrice(candle.open)}  H ${formatPrice(candle.high)}  L ${formatPrice(candle.low)}  C ${formatPrice(candle.close)}`;
}

export function isChartCandle(value: unknown): value is ChartCandle {
	return (
		typeof value === "object" &&
		value !== null &&
		"time" in value &&
		typeof value.time === "number" &&
		"open" in value &&
		"high" in value &&
		"low" in value &&
		"close" in value
	);
}

export function chartPriceResolution(data: readonly ChartCandleSlot[]): {
	base: number;
	fractionDigits: number;
	minMove: number;
} {
	const prices = data.flatMap((item) =>
		"open" in item ? [item.open, item.high, item.low, item.close] : [],
	);
	const smallest = Math.min(...prices.filter((price) => price > 0));
	// Eight significant digits (at least cents), with a valid decimal tick base.
	const exponent = Number.isFinite(smallest)
		? Math.min(308, Math.max(2, 7 - Math.floor(Math.log10(smallest))))
		: 2;
	return {
		base: 10 ** exponent,
		// Intl accepts at most 100 fraction digits.
		fractionDigits: Math.min(100, exponent),
		minMove: 10 ** -exponent,
	};
}
