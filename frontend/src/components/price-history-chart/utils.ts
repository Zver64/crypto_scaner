import type { AutoscaleInfo, IRange, UTCTimestamp } from "lightweight-charts";
import type {
	ChartCandle,
	ChartCandleSlot,
	ChartIndicatorOptions,
	ChartIndicatorPane,
	ChartIndicatorScale,
	ChartIndicatorSlot,
	ChartInterval,
	ChartLegendItem,
	ChartPaneIndicatorOptions,
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
	return valueResolution(
		data.flatMap((item) =>
			"open" in item ? [item.open, item.high, item.low, item.close] : [],
		),
	);
}

// Eight significant digits (at least cents) of the smallest non-zero magnitude,
// with a valid decimal tick base. Indicator panes use it too, so tiny values
// such as MACD of a low-priced coin keep their digits.
export function valueResolution(values: readonly number[]): {
	base: number;
	fractionDigits: number;
	minMove: number;
} {
	const smallest = Math.min(
		...values.map(Math.abs).filter((value) => value > 0),
	);
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

export function formatChartTime(
	value: string | number,
	interval: ChartInterval,
): string {
	const timestamp =
		typeof value === "number" ? value * 1_000 : Date.parse(value);
	const date = new Date(timestamp);
	if (interval === "1h") {
		return `${new Intl.DateTimeFormat("en", { day: "numeric", hour: "2-digit", hourCycle: "h23", minute: "2-digit", month: "short", timeZone: "UTC" }).format(date)} UTC`;
	}
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

export function nextCandleOpen(value: string, interval: ChartInterval): string {
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

// Titles and values of every indicator line at one candle slot. Indicator data
// is aligned with the candle slots, so the slot index selects the value.
export function createIndicatorLegend(
	indicators: readonly ChartIndicatorOptions[],
	indicatorData: ReadonlyMap<string, readonly ChartIndicatorSlot[]>,
	slotIndex: number,
): ChartLegendItem[] {
	return indicators.flatMap((indicator) =>
		indicator.lines.map(({ color, output, title }) => {
			const key = `${indicator.id}:${output}`;
			const slot = indicatorData.get(key)?.[slotIndex];
			const value =
				slot && "value" in slot
					? indicator.placement === "pane"
						? formatNumber(slot.value)
						: formatPrice(slot.value)
					: null;
			return { color, key, placement: indicator.placement, title, value };
		}),
	);
}

// Groups the pane indicators by pane key, ordering the panes by their first
// indicator.
export function createIndicatorPanes(
	indicators: readonly ChartIndicatorOptions[],
): ChartIndicatorPane[] {
	const groups = new Map<string, ChartPaneIndicatorOptions[]>();
	for (const indicator of indicators) {
		if (indicator.placement !== "pane") continue;
		const group = groups.get(indicator.pane);
		if (group) group.push(indicator);
		else groups.set(indicator.pane, [indicator]);
	}
	return [...groups.values()].map((group) => ({
		indicators: group,
		scale: group.map(({ scale }) => scale).reduce(mergeScales),
	}));
}

// The scale of a shared pane spans the bounds of all its indicators, an
// unbounded side staying unbounded. A shared pane draws no levels.
function mergeScales(
	first: ChartIndicatorScale,
	second: ChartIndicatorScale,
): ChartIndicatorScale {
	return {
		levels: [],
		max:
			first.max === undefined || second.max === undefined
				? undefined
				: Math.max(first.max, second.max),
		min:
			first.min === undefined || second.min === undefined
				? undefined
				: Math.min(first.min, second.min),
	};
}

// Fits a pane scale to the visible values while keeping its reference levels,
// such as RSI 30 and 70, in view and staying within the indicator's bounds.
export function fitPaneIndicatorScale(
	visible: AutoscaleInfo | null,
	{ levels, max, min }: Pick<ChartIndicatorScale, "levels" | "max" | "min">,
): AutoscaleInfo | null {
	const values = levels.map(({ value }) => value);
	if (visible?.priceRange) {
		values.push(visible.priceRange.minValue, visible.priceRange.maxValue);
	}
	if (values.length === 0) return visible;
	return {
		...visible,
		priceRange: {
			maxValue: Math.min(Math.max(...values), max ?? Number.POSITIVE_INFINITY),
			minValue: Math.max(Math.min(...values), min ?? Number.NEGATIVE_INFINITY),
		},
	};
}
