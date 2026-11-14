import type {
	AutoscaleInfo,
	AutoscaleInfoProvider,
	IRange,
	PriceFormatCustom,
	UTCTimestamp,
} from "lightweight-charts";

import type {
	ChartIndicatorDefinition,
	ChartPageResponse,
} from "@/api/generated/models";
import {
	entryMarkerOptions,
	exitMarkerOptions,
	indicatorMarkerPositions,
	markerSegmentGapColor,
	markMarkerOptions,
	volumeColors,
} from "@/features/candle-chart/config";
import {
	dayTimeFormat,
	hourTimeFormat,
	monthTimeFormat,
} from "@/features/candle-chart/constants";
import type {
	ChartCandle,
	ChartCandleSlot,
	ChartIndicatorLayout,
	ChartIndicatorLine,
	ChartIndicatorOptions,
	ChartIndicatorPane,
	ChartIndicatorPoints,
	ChartIndicatorScale,
	ChartIndicatorSlot,
	ChartInterval,
	ChartLegendItem,
	ChartLineData,
	ChartMarker,
	ChartMarkerSegments,
	ChartPaneIndicatorOptions,
	ChartPriceLevels,
	ChartSeriesData,
	ChartTradeMarkers,
	ChartVolumeSlot,
	IndicatorPoint,
	PriceCandle,
	ValueResolution,
} from "@/features/candle-chart/types";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

// Results arrive in catalog order. Both chart sources use the same id/output
// mapping.
function chartIndicatorPoints(
	chart: Pick<ChartPageResponse, "indicators"> | undefined,
	catalog: readonly Pick<ChartIndicatorDefinition, "id">[],
): ChartIndicatorPoints {
	if (!chart) return {};
	return Object.fromEntries(
		catalog.map(({ id }, index) => [
			id,
			Object.fromEntries(
				(chart.indicators[index]?.series ?? []).map(({ name, points }) => [
					name,
					points,
				]),
			),
		]),
	);
}

// The chart-ready data of a snapshot, created once by its source: candle
// slots with whitespace for missing candles, volume bars, and every catalog
// line aligned with the slots, so points before the first candle are dropped.
export function createChartSeriesData(
	candles: readonly PriceCandle[],
	chart: Pick<ChartPageResponse, "indicators"> | undefined,
	catalog: readonly Pick<
		ChartIndicatorDefinition,
		"id" | "lines" | "placement"
	>[],
	interval: ChartInterval,
): ChartSeriesData {
	const slots = createCandlestickData(candles, interval);
	const points = chartIndicatorPoints(chart, catalog);
	const priceResolution = chartPriceResolution(slots);
	return {
		candles: slots,
		volumes: createVolumeData(slots, candles, volumeColors),
		priceResolution,
		lines: Object.fromEntries(
			catalog.flatMap(({ id, lines, placement }) =>
				lines.map(({ output }) => {
					const data = createIndicatorData(slots, points[id]?.[output] ?? []);
					// Overlays are drawn on the candles' scale, so only other lines
					// have a resolution of their own.
					const line: ChartLineData = {
						data,
						resolution:
							placement === "overlay"
								? priceResolution
								: valueResolution(
										data.flatMap((slot) =>
											"value" in slot ? [slot.value] : [],
										),
									),
					};
					return [`${id}:${output}`, line];
				}),
			),
		),
	};
}

// Catalog presentation is shared by live charts and immutable backtest charts.
function toChartIndicatorOptions({
	id,
	lines,
	pane,
	placement,
	scale,
}: ChartIndicatorDefinition): ChartIndicatorOptions {
	return placement === "pane"
		? {
				id,
				lines,
				placement,
				pane: pane ?? id,
				scale: scale ?? { levels: [] },
			}
		: { id, lines, placement };
}

// What a chart draws from one interval's catalog, created once per catalog.
export function createIndicatorLayout(
	catalog: readonly ChartIndicatorDefinition[],
): ChartIndicatorLayout {
	const indicators = catalog.map(toChartIndicatorOptions);
	return { indicators, panes: createIndicatorPanes(indicators) };
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
// trades of a backtest, and the overlay lines drawn as markers without an
// anchor. One series stacks only the markers of one ascending list, so they
// are merged; without marker lines the markers of the source are returned as
// they are.
export function createCandleMarkers(
	markers: readonly ChartMarker[] | undefined,
	indicators: readonly ChartIndicatorOptions[],
	lines: Readonly<Record<string, ChartLineData>>,
	themeColor: (color: string) => string,
): readonly ChartMarker[] | undefined {
	const indicatorMarkers = indicators.flatMap(
		({ id, lines: indicatorLines, placement }) =>
			placement === "overlay"
				? indicatorLines.flatMap((line) =>
						line.anchor ? [] : lineMarkers(id, line, lines, themeColor),
					)
				: [],
	);
	if (indicatorMarkers.length === 0) return markers;
	return [...(markers ?? []), ...indicatorMarkers].sort(
		(first, second) => first.time - second.time,
	);
}

// The markers of the lines anchored to a line of another indicator, by the
// "id:output" key of that line, whose series draws them wherever it is
// placed. A series has markers only where it has values, so the others are
// dropped, and one ascending list of them, so those of several lines are
// merged.
export function createAnchoredMarkers(
	indicators: readonly ChartIndicatorOptions[],
	lines: Readonly<Record<string, ChartLineData>>,
	themeColor: (color: string) => string,
): Readonly<Record<string, readonly ChartMarker[]>> {
	const markers: Record<string, ChartMarker[]> = {};
	for (const { id, lines: indicatorLines } of indicators) {
		for (const line of indicatorLines) {
			if (!line.anchor) continue;
			const key = `${line.anchor.indicator_id}:${line.anchor.output}`;
			const anchored = new Set(
				(lines[key]?.data ?? []).flatMap((slot) =>
					"value" in slot ? [slot.time] : [],
				),
			);
			markers[key] = [
				...(markers[key] ?? []),
				...lineMarkers(id, line, lines, themeColor).filter(({ time }) =>
					anchored.has(time),
				),
			].sort((first, second) => first.time - second.time);
		}
	}
	return markers;
}

// The segments between marker values: where a connected time falls on a candle
// slot holding a value of a marker line, that value is connected with the
// previous value of the line, both where their markers are drawn. The segments
// lie where the markers sit: on the candles, or in the pane and on the scale of
// the anchor. An anchored line also connects the same two candles, from low to
// low under them or from high to high over them.
export function createMarkerSegments(
	connected: readonly string[] | undefined,
	{ indicators, panes }: ChartIndicatorLayout,
	{ candles, lines }: Pick<ChartSeriesData, "candles" | "lines">,
	interval: ChartInterval,
): ChartMarkerSegments[] {
	if (!connected || connected.length === 0) return [];
	const times = new Set<number>(
		connected.map((time) => candleOpenTime(time, interval)),
	);
	return indicators.flatMap(({ id, lines: indicatorLines, placement }) =>
		indicatorLines.flatMap(
			({ anchor, color, output, shift, style }): ChartMarkerSegments[] => {
				if (style === "line") return [];
				const key = `${id}:${output}`;
				const data = lines[key]?.data ?? [];
				// The slots of the two values of each segment.
				const pairs: [number, number][] = [];
				let previous: number | undefined;
				data.forEach((slot, index) => {
					if (!("value" in slot)) return;
					if (previous !== undefined && times.has(slot.time)) {
						pairs.push([previous, index]);
					}
					previous = index;
				});
				const ownPoint = (index: number) => {
					const slot = data[index];
					const time = data[index - shift]?.time;
					return slot && "value" in slot && time !== undefined
						? { time, value: slot.value }
						: undefined;
				};
				const candlePoint = (index: number) => {
					const candle = candles[index - shift];
					return candle && "open" in candle
						? {
								time: candle.time,
								value: style === "marker_below" ? candle.low : candle.high,
							}
						: undefined;
				};
				const segments: ChartMarkerSegments[] = [];
				if (!anchor) {
					if (placement !== "overlay") return [];
					segments.push({
						color,
						data: segmentData(pairs, ownPoint),
						key,
						pane: 0,
						scale: "price",
					});
				} else {
					const pane = panes.findIndex(({ indicators: paneIndicators }) =>
						paneIndicators.some(
							(indicator) => indicator.id === anchor.indicator_id,
						),
					);
					const anchorPlacement = indicators.find(
						(indicator) => indicator.id === anchor.indicator_id,
					)?.placement;
					if (anchorPlacement !== undefined) {
						segments.push({
							color,
							data: segmentData(pairs, ownPoint),
							key,
							pane: pane + 1,
							scale: anchorPlacement === "volume" ? "volume" : "price",
						});
					}
					segments.push({
						color,
						data: segmentData(pairs, candlePoint),
						key: `${key}:candles`,
						pane: 0,
						scale: "price",
					});
				}
				return segments.filter(({ data }) => data.length > 0);
			},
		),
	);
}

// The points of segments in ascending order as the data of one line series:
// segments sharing a point meet there, and the end of a segment followed by a
// gap colors the join to the next segment transparent. A segment missing a
// point is not drawn.
function segmentData(
	pairs: readonly (readonly [number, number])[],
	point: (index: number) => ChartMarkerSegments["data"][number] | undefined,
): ChartMarkerSegments["data"] {
	const data: ChartMarkerSegments["data"][number][] = [];
	for (const [from, to] of pairs) {
		const start = point(from);
		const end = point(to);
		if (!start || !end) continue;
		const last = data.at(-1);
		if (last?.time !== start.time) {
			if (last)
				data[data.length - 1] = { ...last, color: markerSegmentGapColor };
			data.push(start);
		}
		data.push(end);
	}
	return data;
}

// The markers of a line drawn as markers: a circle for each value, moved the
// line's shift of candle slots earlier. A marker moved before the first slot
// is not drawn.
function lineMarkers(
	id: string,
	{ color, output, shift, style }: ChartIndicatorLine,
	lines: Readonly<Record<string, ChartLineData>>,
	themeColor: (color: string) => string,
): ChartMarker[] {
	if (style === "line") return [];
	const data = lines[`${id}:${output}`]?.data ?? [];
	return data.flatMap(({ time }, index): ChartMarker[] =>
		drawnValue(data, { shift, style }, index) === undefined
			? []
			: [
					{
						color: themeColor(color),
						position: indicatorMarkerPositions[style],
						shape: "circle",
						time,
					},
				],
	);
}

// The value of a line drawn at a candle slot: the value of that slot, or for
// a line drawn as markers the value held the line's shift of slots later,
// whose marker is drawn here.
function drawnValue(
	data: readonly ChartIndicatorSlot[],
	{ shift, style }: Pick<ChartIndicatorLine, "shift" | "style">,
	slotIndex: number,
): number | undefined {
	const slot = data[style === "line" ? slotIndex : slotIndex + shift];
	return slot && "value" in slot ? slot.value : undefined;
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

// Titles and values of every indicator line at one candle slot. Indicator data
// is aligned with the candle slots, so the slot index selects the value; a
// line drawn as markers shows the value of the marker drawn on the slot.
export function createIndicatorLegend(
	indicators: readonly ChartIndicatorOptions[],
	lines: Readonly<Record<string, ChartLineData>>,
	slotIndex: number,
): ChartLegendItem[] {
	return indicators.flatMap((indicator) =>
		indicator.lines.map((line) => {
			const { color, output, title } = line;
			const key = `${indicator.id}:${output}`;
			const drawn = drawnValue(lines[key]?.data ?? [], line, slotIndex);
			const value =
				drawn === undefined
					? null
					: indicator.placement === "overlay"
						? formatPrice(drawn)
						: formatNumber(drawn);
			return { color, key, placement: indicator.placement, title, value };
		}),
	);
}

// Groups the pane indicators by pane key, ordering the panes by their first
// indicator. Each pane gets its autoscale provider here, once per catalog, so
// the provider keeps its identity as long as the layout does.
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
	return [...groups.values()].map((group) => {
		const scale = group.map(({ scale }) => scale).reduce(mergeScales);
		return {
			autoscaleInfoProvider: paneScaleProvider(scale),
			indicators: group,
			scale,
		};
	});
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
