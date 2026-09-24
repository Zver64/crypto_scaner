import type {
	ChartCandleSlot,
	ChartCandlestick,
	ChartIndicatorSlot,
} from "@/components/lightweight-chart";
import type {
	ChartInterval,
	IndicatorPoint,
	PriceCandle,
} from "@/components/price-history-chart/types";
import { formatNumber } from "@/utils/number-format";

export function toUtcTimestamp(value: string): number {
	return Date.parse(value) / 1_000;
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

export function formatPrice(value: number): string {
	return formatNumber(value);
}

export function formatOhlc(candle: PriceCandle | ChartCandlestick): string {
	return `O ${formatPrice(candle.open)}  H ${formatPrice(candle.high)}  L ${formatPrice(candle.low)}  C ${formatPrice(candle.close)}`;
}

export function isChartCandle(value: unknown): value is ChartCandlestick {
	return (
		typeof value === "object" &&
		value !== null &&
		"open" in value &&
		"high" in value &&
		"low" in value &&
		"close" in value
	);
}
