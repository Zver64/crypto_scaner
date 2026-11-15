import { alpha } from "@mantine/core";
import equal from "fast-deep-equal";
import type {
	AutoscaleInfo,
	AutoscaleInfoProvider,
	IRange,
	LineData,
	PriceFormatCustom,
	UTCTimestamp,
} from "lightweight-charts";

import type {
	ChartAreaShape,
	ChartIndicatorLevel,
	ChartIndicatorScale,
	ChartLineShape,
	ChartMarkersShape,
	ChartPageResponse,
	ChartPane,
	ChartSegmentsShape,
} from "@/api/generated/models";
import type { SeriesFillPoint } from "@/components/lightweight-chart";
import {
	areaFillOpacity,
	entryMarkerOptions,
	exitMarkerOptions,
	lineGapColor,
	markMarkerOptions,
	shapeMarkerPositions,
	volumeColors,
} from "@/features/candle-chart/config";
import {
	dayTimeFormat,
	hourTimeFormat,
	monthTimeFormat,
} from "@/features/candle-chart/constants";
import type {
	ChartAreaShapeData,
	ChartCandle,
	ChartCandleSlot,
	ChartIndicatorSlot,
	ChartInterval,
	ChartLegendItem,
	ChartLineData,
	ChartLineShapeData,
	ChartMarker,
	ChartMarkersShapeData,
	ChartPaneData,
	ChartPriceLevels,
	ChartSegmentsShapeData,
	ChartSeriesData,
	ChartShapeData,
	ChartShapePlace,
	ChartTradeMarkers,
	ChartVolumeSlot,
	IndicatorPoint,
	PriceCandle,
	ValueResolution,
} from "@/features/candle-chart/types";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

// The chart-ready data of a snapshot, created once by its source: candle
// slots with whitespace for missing candles, volume bars, the panes, and every
// shape aligned with the slots, so what lies before the first candle is
// dropped. The panes of the previous data keep their autoscale while their
// scales do not change.
export function createChartSeriesData(
	candles: readonly PriceCandle[],
	chart: Pick<ChartPageResponse, "panes" | "shapes"> | undefined,
	interval: ChartInterval,
	previous?: Pick<ChartSeriesData, "panes">,
): ChartSeriesData {
	const slots = createCandlestickData(candles, interval);
	const priceResolution = chartPriceResolution(slots);
	const panes = createChartPanes(chart?.panes ?? [], previous?.panes ?? []);
	// The first line of each pane draws its levels.
	const leveled = new Set<number>();
	const shapes = (chart?.shapes ?? []).flatMap((shape): ChartShapeData[] => {
		const place = shapePlace(shape, panes);
		if (!place) return [];
		switch (shape.kind) {
			case "line": {
				const levels =
					place.pane > 0 && !leveled.has(place.pane)
						? (panes[place.pane - 1]?.scale.levels ?? [])
						: [];
				leveled.add(place.pane);
				return [
					createLineShapeData(slots, shape, place, priceResolution, levels),
				];
			}
			case "area":
				return [createAreaShapeData(slots, shape, place)];
			case "markers":
				return [createMarkersShapeData(slots, shape, place)];
			case "segments":
				return [createSegmentsShapeData(slots, shape, place)];
		}
		return [];
	});
	return {
		candles: slots,
		volumes: createVolumeData(slots, candles, volumeColors),
		priceResolution,
		panes,
		shapes,
	};
}

// The panes below the candles; a pane of the previous data with the same key
// and scale is kept, with its autoscale.
function createChartPanes(
	panes: readonly ChartPane[],
	previous: readonly ChartPaneData[],
): ChartPaneData[] {
	return panes.map(
		({ key, scale }) =>
			previous.find((pane) => pane.key === key && equal(pane.scale, scale)) ?? {
				autoscaleInfoProvider: paneScaleProvider(scale),
				key,
				scale,
			},
	);
}

// Where a shape is drawn; a shape in a pane the page does not have is not
// drawn.
function shapePlace(
	{ pane, placement }: Pick<ChartLineShape, "pane" | "placement">,
	panes: readonly ChartPaneData[],
): ChartShapePlace | undefined {
	if (placement !== "pane") return { pane: 0, placement };
	const index = panes.findIndex(({ key }) => key === pane);
	const found = panes[index];
	return found
		? {
				autoscaleInfoProvider: found.autoscaleInfoProvider,
				pane: index + 1,
				placement,
			}
		: undefined;
}

// A line aligned with the candle slots. A point that is not joined to the
// point before leaves out the join from that point.
export function createLineShapeData(
	slots: readonly ChartCandleSlot[],
	{ color, id, points, title }: ChartLineShape,
	place: ChartShapePlace,
	priceResolution: ValueResolution,
	levels: readonly ChartIndicatorLevel[],
): ChartLineShapeData {
	const byTime = new Map(
		points.map((point) => [toUtcTimestamp(point.time), point]),
	);
	const data = slots.map(({ time }): ChartIndicatorSlot => {
		const point = byTime.get(time);
		if (!point) return { time };
		return point.color === undefined
			? { time, value: point.value }
			: { color: point.color, time, value: point.value };
	});
	const gaps = new Set(
		points.flatMap((point, index) => {
			const before = points[index - 1];
			return point.break && before ? [toUtcTimestamp(before.time)] : [];
		}),
	);
	return {
		...place,
		color,
		data,
		gaps,
		id,
		kind: "line",
		levels,
		// Overlays are drawn on the candles' scale, so only other lines have a
		// resolution of their own.
		resolution:
			place.placement === "overlay"
				? priceResolution
				: valueResolution(
						data.flatMap((slot) => ("value" in slot ? [slot.value] : [])),
					),
		title,
	};
}

// The data of a line as drawn: point colors resolved, and the joins it
// leaves out transparent.
export function lineSeriesData(
	{ data, gaps }: Pick<ChartLineShapeData, "data" | "gaps">,
	themeColor: (color: string) => string,
): ChartIndicatorSlot[] {
	return data.map((slot) => {
		if (!("value" in slot)) return slot;
		if (gaps.has(slot.time)) return { ...slot, color: lineGapColor };
		return slot.color === undefined
			? slot
			: { ...slot, color: themeColor(slot.color) };
	});
}

// An area aligned with the candle slots, with its values as the data of the
// series it lies on.
export function createAreaShapeData(
	slots: readonly ChartCandleSlot[],
	{ id, points }: ChartAreaShape,
	place: ChartShapePlace,
): ChartAreaShapeData {
	const byTime = new Map(
		points.map((point) => [toUtcTimestamp(point.time), point]),
	);
	const aligned = slots.map(({ time }) => ({ point: byTime.get(time), time }));
	return {
		...place,
		id,
		kind: "area",
		points: aligned.map(({ point, time }): SeriesFillPoint =>
			point
				? { base: point.base, color: point.color, time, value: point.value }
				: { time },
		),
		values: aligned.map(({ point, time }) =>
			point ? { time, value: point.value } : { time },
		),
	};
}

// The fill of an area as drawn: translucent in the resolved color of each
// point.
export function areaFillPoints(
	points: readonly SeriesFillPoint[],
	themeColor: (color: string) => string,
): SeriesFillPoint[] {
	return points.map((point) =>
		"value" in point
			? { ...point, color: alpha(themeColor(point.color), areaFillOpacity) }
			: point,
	);
}

// Markers on the candle slots, split into those on a line and those on the
// candles. A marker on a candle the slots do not have is not drawn.
export function createMarkersShapeData(
	slots: readonly ChartCandleSlot[],
	{ color, id, points, position, title }: ChartMarkersShape,
	place: ChartShapePlace,
): ChartMarkersShapeData {
	const candles = new Set(
		slots.flatMap((slot) => ("open" in slot ? [slot.time] : [])),
	);
	const values = new Map<UTCTimestamp, number>();
	const onLine = new Map<UTCTimestamp, number>();
	const onCandles = new Set<UTCTimestamp>();
	for (const point of points) {
		const time = toUtcTimestamp(point.time);
		if (!candles.has(time) || values.has(time)) continue;
		values.set(time, point.value);
		if (point.line_value === undefined) onCandles.add(time);
		else onLine.set(time, point.line_value);
	}
	return {
		...place,
		color,
		id,
		kind: "markers",
		onCandles: [...onCandles].sort((first, second) => first - second),
		onLine: [...onLine]
			.map(([time, value]) => ({ time, value }))
			.sort((first, second) => first.time - second.time),
		position: shapeMarkerPositions[position],
		title,
		values,
	};
}

// The markers drawn on a line, in the resolved color of their shape.
export function lineMarkers(
	{ color, onLine, position }: ChartMarkersShapeData,
	themeColor: (color: string) => string,
): ChartMarker[] {
	return onLine.map(({ time }) =>
		shapeMarker(time, color, position, themeColor),
	);
}

// The segments on the candle slots as the data of one line series. A segment
// with a point on a candle the slots do not have, or starting before the end
// of the one before, is not drawn.
export function createSegmentsShapeData(
	slots: readonly ChartCandleSlot[],
	{ color, id, segments }: ChartSegmentsShape,
	place: ChartShapePlace,
): ChartSegmentsShapeData {
	const candles = new Set(
		slots.flatMap((slot) => ("open" in slot ? [slot.time] : [])),
	);
	const data: LineData<UTCTimestamp>[] = [];
	for (const segment of segments) {
		const start = {
			time: toUtcTimestamp(segment.from.time),
			value: segment.from.value,
		};
		const end = {
			time: toUtcTimestamp(segment.to.time),
			value: segment.to.value,
		};
		const last = data.at(-1);
		if (
			!candles.has(start.time) ||
			!candles.has(end.time) ||
			(last && start.time < last.time)
		)
			continue;
		if (last?.time !== start.time) {
			if (last) data[data.length - 1] = { ...last, color: lineGapColor };
			data.push(start);
		}
		data.push(end);
	}
	return { ...place, color, data, id, kind: "segments" };
}

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

// The close prices of another instrument's candles on the candle slots of the
// chart; candles without a slot are dropped, so the line adds no bars.
export function createComparisonLine(
	slots: readonly ChartCandleSlot[],
	candles: readonly PriceCandle[],
): ChartLineData {
	return {
		data: createIndicatorData(
			slots,
			candles.map(({ close, open_time }) => ({
				time: open_time,
				value: close,
			})),
		),
		resolution: valueResolution(candles.map(({ close }) => close)),
	};
}

// Aggregate trades once per selection/interval. Each source interval owns its
// selector; candle arrays with only OHLC changes reuse the marker array.
export function createMarkerDataSelector(
	{ entries, exits, marks = [], groups = [] }: ChartTradeMarkers,
	interval: ChartInterval,
) {
	const entryCounts = countByCandle(entries, interval);
	const exitCounts = countByCandle(exits, interval);
	const markCounts = countByCandle(marks, interval);
	const groupedCounts = groups.map(({ times, options }) => ({
		counts: countByCandle(times, interval),
		options,
	}));
	let previous: readonly ChartCandleSlot[] = [];
	let result: ChartMarker[] = [];
	return (data: readonly ChartCandleSlot[]): ChartMarker[] => {
		if (
			data.length === previous.length &&
			data.every((slot, index) => {
				const old = previous[index];
				return (
					old !== undefined &&
					old.time === slot.time &&
					"open" in old === "open" in slot
				);
			})
		) {
			return result;
		}
		previous = data;
		result = data.flatMap((slot) =>
			"open" in slot
				? [
						...candleMarker(
							entryMarkerOptions,
							entryCounts.get(slot.time),
							slot.time,
						),
						...candleMarker(
							exitMarkerOptions,
							exitCounts.get(slot.time),
							slot.time,
						),
						...candleMarker(
							markMarkerOptions,
							markCounts.get(slot.time),
							slot.time,
						),
						...groupedCounts.flatMap(({ counts, options }) =>
							candleMarker(options, counts.get(slot.time), slot.time),
						),
					]
				: [],
		);
		return result;
	};
}

// Everything marked on the candles: the markers of the source, such as the
// trades of a backtest, and the markers of shapes drawn on the candles. One
// series stacks only the markers of one ascending list, so they are merged;
// without such shape markers the markers of the source are returned as they
// are.
export function createCandleMarkers(
	markers: readonly ChartMarker[] | undefined,
	shapes: readonly ChartShapeData[],
	themeColor: (color: string) => string,
): readonly ChartMarker[] | undefined {
	const shapeMarkers = shapes.flatMap((shape) =>
		shape.kind === "markers"
			? shape.onCandles.map((time) =>
					shapeMarker(time, shape.color, shape.position, themeColor),
				)
			: [],
	);
	if (shapeMarkers.length === 0) return markers;
	return [...(markers ?? []), ...shapeMarkers].sort(
		(first, second) => first.time - second.time,
	);
}

function shapeMarker(
	time: UTCTimestamp,
	color: string,
	position: ChartMarker["position"],
	themeColor: (color: string) => string,
): ChartMarker {
	return { color: themeColor(color), position, shape: "circle", time };
}

function countByCandle(
	times: readonly string[],
	interval: ChartInterval,
): Map<number, number> {
	const counts = new Map<number, number>();
	for (const time of times) {
		const open = candleOpenTime(time, interval);
		counts.set(open, (counts.get(open) ?? 0) + 1);
	}
	return counts;
}

function candleMarker(
	options: Omit<ChartMarker, "time">,
	count: number | undefined,
	time: UTCTimestamp,
): ChartMarker[] {
	if (count === undefined) return [];
	return [
		count > 1
			? { ...options, text: String(count), time }
			: { ...options, time },
	];
}

// Open time of the candle containing the given time, on the exchange's UTC
// boundaries: weeks start on Monday, months on the first day.
function candleOpenTime(value: string, interval: ChartInterval): UTCTimestamp {
	const date = new Date(value);
	if (interval === "1h") {
		date.setUTCMinutes(0, 0, 0);
		return (date.getTime() / 1_000) as UTCTimestamp;
	}
	date.setUTCHours(0, 0, 0, 0);
	if (interval === "1w") {
		date.setUTCDate(date.getUTCDate() - ((date.getUTCDay() + 6) % 7));
	} else if (interval === "1M") {
		date.setUTCDate(1);
	}
	return (date.getTime() / 1_000) as UTCTimestamp;
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

export function formatCandleRange(
	candle: Pick<PriceCandle, "open" | "high" | "low">,
): string {
	return formatRangePercent(((candle.high - candle.low) / candle.open) * 100);
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

export function chartPriceResolution(
	data: readonly ChartCandleSlot[],
): ValueResolution {
	return valueResolution(
		data.flatMap((item) =>
			"open" in item ? [item.open, item.high, item.low, item.close] : [],
		),
	);
}

// Eight significant digits (at least cents) of the smallest non-zero magnitude,
// with a valid decimal tick base. Indicator panes use it too, so tiny values
// such as MACD of a low-priced coin keep their digits.
export function valueResolution(values: readonly number[]): ValueResolution {
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

// The axis price format of a resolution. Axis ticks carry floating-point noise
// near zero; rounding it at the resolution keeps it from widening the scale.
export function axisPriceFormat({
	base,
	fractionDigits,
	minMove,
}: ValueResolution): PriceFormatCustom {
	return {
		base,
		formatter: (value: number) => formatNumber(value, fractionDigits),
		minMove,
		type: "custom",
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
		return `${hourTimeFormat.format(date)} UTC`;
	}
	if (interval === "1M") {
		return `${monthTimeFormat.format(date)} UTC`;
	}
	const day = dayTimeFormat.format(date);
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

// Titles and values of the shapes with a title at one candle slot: a line
// shows its value there in the color of its point, markers the value of the
// marker drawn on that candle.
export function createIndicatorLegend(
	{ candles, shapes }: Pick<ChartSeriesData, "candles" | "shapes">,
	slotIndex: number,
): ChartLegendItem[] {
	const time = candles[slotIndex]?.time;
	return shapes.flatMap((shape): ChartLegendItem[] => {
		if (shape.kind !== "line" && shape.kind !== "markers") return [];
		const { color, id, placement, title } = shape;
		const format = (value: number | undefined) =>
			value === undefined
				? null
				: placement === "overlay"
					? formatPrice(value)
					: formatNumber(value);
		if (shape.kind === "markers") {
			return [
				{
					color,
					key: id,
					placement,
					title,
					value: format(
						time === undefined ? undefined : shape.values.get(time),
					),
				},
			];
		}
		const slot = shape.data[slotIndex];
		return [
			{
				color: (slot && "color" in slot ? slot.color : undefined) ?? color,
				key: id,
				placement,
				title,
				value: format(slot && "value" in slot ? slot.value : undefined),
			},
		];
	});
}

// Fits a scale to the visible values while keeping its levels, such as RSI 30
// and 70 or the bounds of price levels, in view and staying within its bounds.
export function fitScaleToLevels(
	visible: AutoscaleInfo | null,
	{
		levels,
		max,
		min,
	}: Pick<ChartIndicatorScale, "max" | "min"> & {
		levels: readonly { value: number }[];
	},
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

// Fits the pane to its visible values instead of the full range, such as RSI
// 0-100, so the line fills the pane without empty space above and below.
function paneScaleProvider(scale: ChartIndicatorScale): AutoscaleInfoProvider {
	return (baseImplementation) => fitScaleToLevels(baseImplementation(), scale);
}

// Keeps the bounds of the price levels in view. The autoscale provider is
// created with the levels, so it keeps its identity while they do not change.
export function createChartPriceLevels(
	levels: readonly number[],
): ChartPriceLevels {
	const bounds = [levels[0], levels[levels.length - 1]].map((value) => ({
		value,
	}));
	return {
		autoscaleInfoProvider: (baseImplementation) =>
			fitScaleToLevels(baseImplementation(), { levels: bounds }),
		levels,
	};
}
