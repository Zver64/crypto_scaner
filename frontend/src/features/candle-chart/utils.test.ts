import type { UTCTimestamp } from "lightweight-charts";
import { describe, expect, it, vi } from "vitest";

import type {
	ChartLineShape,
	ChartMarker,
	ChartMarkersShape,
	ChartPane,
	ChartSegmentsShape,
	ChartShape,
} from "@/api/generated/models";
import {
	areaFillPoints,
	createCandleMarkers,
	createCandlestickData,
	createChartSeriesData,
	createIndicatorData,
	createIndicatorLegend,
	createMarkerDataSelector,
	fitScaleToLevels,
	formatCandleRange,
	formatChartTime,
	formatOhlc,
	formatPrice,
	getVisibleMinMax,
	lastVisibleCandleIndex,
	lineMarkers,
	lineSeriesData,
	nextCandleOpen,
	withMinMaxMarkers,
} from "@/features/candle-chart/utils";

const from = "2026-08-26T23:00:00Z";

function candle(slot: number, close: number) {
	return {
		close,
		high: close + 1,
		low: close - 1,
		open: close - 0.5,
		volume: 100,
		open_time: new Date(Date.parse(from) + slot * 3_600_000).toISOString(),
	};
}

describe("price history chart data", () => {
	it("preserves missing candle slots without changing observed candle times", () => {
		expect(createCandlestickData([candle(0, 10), candle(2, 12)], "1h")).toEqual(
			[
				{ close: 10, high: 11, low: 9, open: 9.5, time: 1_787_785_200 },
				{ time: 1_787_788_800 },
				{ close: 12, high: 13, low: 11, open: 11.5, time: 1_787_792_400 },
			],
		);
	});

	it("aligns an indicator on the existing grid, including gaps and warm-up slots", () => {
		const candles = [candle(0, 10), candle(2, 12), candle(3, 13)];
		const data = createCandlestickData(candles, "1h");
		expect(
			createIndicatorData(data, [
				{ time: candles[1].open_time, value: 45 },
				{ time: candles[2].open_time, value: 55 },
			]),
		).toEqual([
			{ time: 1_787_785_200 },
			{ time: 1_787_788_800 },
			{ time: 1_787_792_400, value: 45 },
			{ time: 1_787_796_000, value: 55 },
		]);
	});

	it("marks the loaded candles containing the trade times and counts several", () => {
		const weeks = createCandlestickData(
			[
				{ ...candle(0, 10), open_time: "2026-08-24T00:00:00Z" },
				{ ...candle(0, 10), open_time: "2026-08-31T00:00:00Z" },
			],
			"1w",
		);
		// Sunday 23:00 closes the first week; Monday 00:00 opens the second.
		expect(
			createMarkerDataSelector(
				{
					entries: [
						"2026-08-17T05:00:00Z",
						"2026-08-24T00:00:00Z",
						"2026-08-30T23:00:00Z",
						"2026-08-31T00:00:00Z",
					],
					exits: ["2026-08-25T00:00:00Z"],
				},
				"1w",
			)(weeks).map(({ position, text, time }) => ({ position, text, time })),
		).toEqual([
			{
				position: "belowBar",
				text: "2",
				time: Date.parse("2026-08-24T00:00:00Z") / 1_000,
			},
			{
				position: "aboveBar",
				text: undefined,
				time: Date.parse("2026-08-24T00:00:00Z") / 1_000,
			},
			{
				position: "belowBar",
				text: undefined,
				time: Date.parse("2026-08-31T00:00:00Z") / 1_000,
			},
		]);
		const months = createCandlestickData(
			[{ ...candle(0, 10), open_time: "2026-08-01T00:00:00Z" }],
			"1M",
		);
		expect(
			createMarkerDataSelector(
				{
					entries: [],
					exits: ["2026-07-31T23:00:00Z", "2026-08-31T23:00:00Z"],
				},
				"1M",
			)(months).map(({ text, time }) => ({ text, time })),
		).toEqual([
			{ text: undefined, time: Date.parse("2026-08-01T00:00:00Z") / 1_000 },
		]);
	});

	it("does not insert duplicate slots for equivalent ISO timestamp formats", () => {
		const first = candle(0, 10);
		const second = candle(1, 11);
		first.open_time = first.open_time.replace(".000Z", "Z");
		second.open_time = second.open_time.replace(".000Z", "Z");
		expect(
			createCandlestickData([first, second], "1h").map(({ time }) => time),
		).toEqual([1_787_785_200, 1_787_788_800]);
	});

	it("formats OHLC with useful price precision", () => {
		expect(formatPrice(0.0000123456789)).toBe("0.00001235");
		expect(formatOhlc({ open: 1, high: 2, low: 0.5, close: 1.5 })).toBe(
			"O 1  H 2  L 0.5  C 1.5",
		);
	});

	it("formats the candle range relative to its open", () => {
		expect(formatCandleRange({ open: 1, high: 2, low: 0.5 })).toBe("150%");
	});

	it("formats UTC labels for each interval", () => {
		expect(formatChartTime("2026-08-26T23:00:00Z", "1h")).toBe(
			"Aug 26, 23:00 UTC",
		);
		expect(formatChartTime("2026-08-26T23:00:00Z", "1w")).toContain("Week of");
	});

	it("advances each interval to the next candle open", () => {
		expect(nextCandleOpen("2026-01-01T00:00:00Z", "1M")).toBe(
			"2026-02-01T00:00:00.000Z",
		);
		expect(nextCandleOpen("2026-08-26T23:00:00Z", "1h")).toBe(
			"2026-08-27T00:00:00.000Z",
		);
	});
});

describe("marker data selector", () => {
	it("aggregates times within a group without merging different outcomes", () => {
		const candles = [candle(0, 10), candle(1, 11)];
		const times = candles.map(({ open_time }) => open_time);
		const options = {
			position: "aboveBar",
			shape: "arrowDown",
			size: 2,
		} as const;
		const select = createMarkerDataSelector(
			{
				entries: [],
				exits: [],
				groups: [
					{
						times: [times[1], times[0], times[0]],
						options: { ...options, color: "green" },
					},
					{ times: [times[0]], options: { ...options, color: "red" } },
				],
			},
			"1h",
		);
		const data = createCandlestickData(candles, "1h");
		expect(
			select(data).map(({ time, color, text }) => ({ time, color, text })),
		).toEqual([
			{ time: data[0].time, color: "green", text: "2" },
			{ time: data[0].time, color: "red", text: undefined },
			{ time: data[1].time, color: "green", text: undefined },
		]);
		expect(select([{ time: data[0].time }, data[1]])).toHaveLength(1);
	});

	it("reuses markers on OHLC updates and aggregates trades only once", () => {
		const candles = [candle(0, 10), candle(1, 11)];
		const entries = candles.map(({ open_time }) => open_time);
		const iterate = vi.spyOn(entries, Symbol.iterator);
		const select = createMarkerDataSelector({ entries, exits: [] }, "1h");
		const data = createCandlestickData(candles, "1h");
		const markers = select(data);
		const updated = createCandlestickData(
			candles.map((item) => ({ ...item, close: 20, volume: 200 })),
			"1h",
		);
		expect(select(updated)).toBe(markers);
		select(createCandlestickData([...candles, candle(2, 12)], "1h"));
		expect(iterate).toHaveBeenCalledTimes(1);
		iterate.mockRestore();
	});

	it("detects added, removed, replaced interior slots and candle/gap changes", () => {
		const candles = [0, 1, 2, 3].map((index) => candle(index, 10));
		const data = createCandlestickData(candles, "1h");
		const select = createMarkerDataSelector(
			{ entries: candles.map(({ open_time }) => open_time), exits: [] },
			"1h",
		);
		const original = [data[0], data[1], data[3]];
		const markers = select(original);
		expect(select(data).map(({ time }) => time)).toEqual(
			data.map(({ time }) => time),
		);
		expect(select(original).map(({ time }) => time)).toEqual(
			original.map(({ time }) => time),
		);
		const replaced = [data[0], data[2], data[3]];
		expect(select(replaced)).not.toBe(markers);
		expect(select(replaced).map(({ time }) => time)).toEqual(
			replaced.map(({ time }) => time),
		);
		const gap = [data[0], { time: data[2].time }, data[3]];
		expect(select(gap).map(({ time }) => time)).toEqual([
			data[0].time,
			data[3].time,
		]);
		expect(select(replaced).map(({ time }) => time)).toEqual(
			replaced.map(({ time }) => time),
		);
		expect(select([])).toEqual([]);
		expect(select(original).map(({ time }) => time)).toEqual(
			original.map(({ time }) => time),
		);
	});

	it("uses the new trades and interval when a selector is replaced", () => {
		const candles = [candle(1, 10), candle(2, 11)];
		const data = createCandlestickData(candles, "1h");
		const entries = candles.map(({ open_time }) => open_time);
		const first = createMarkerDataSelector(
			{ entries: [entries[0]], exits: [] },
			"1h",
		);
		const changed = createMarkerDataSelector(
			{ entries: [], exits: [entries[1]] },
			"1h",
		);
		expect(
			first(data).map(({ time, position }) => ({ time, position })),
		).toEqual([{ time: data[0].time, position: "belowBar" }]);
		expect(
			changed(data).map(({ time, position }) => ({ time, position })),
		).toEqual([{ time: data[1].time, position: "aboveBar" }]);
		const daily = createMarkerDataSelector({ entries, exits: [] }, "1d");
		const days = createCandlestickData(
			[{ ...candles[0], open_time: "2026-08-27T00:00:00Z" }],
			"1d",
		);
		expect(daily(days).map(({ time, text }) => ({ time, text }))).toEqual([
			{ time: days[0].time, text: "2" },
		]);
	});
});

describe("shape data", () => {
	// Slots 0-4 with the candle of slot 3 missing.
	const candles = [0, 1, 2, 4].map((slot) => candle(slot, 10));
	const slots = createCandlestickData(candles, "1h");
	const times = slots.map(({ time }) => time);
	const at = (slot: number) => candle(slot, 0).open_time;
	const color = (token: string) => `resolved ${token}`;
	const rsiPane = {
		key: "rsi",
		scale: { levels: [{ title: "Overbought", value: 70 }], max: 100, min: 0 },
	};
	const line = (
		id: string,
		points: ChartLineShape["points"],
		place: Pick<ChartLineShape, "pane" | "placement"> = {
			pane: "rsi",
			placement: "pane",
		},
	): ChartLineShape => ({
		...place,
		color: "blue.5",
		id,
		indicator: "0",
		kind: "line",
		points,
		title: id.toUpperCase(),
	});
	const series = (
		shapes: ChartShape[],
		panes: ChartPane[] = [rsiPane],
		previous?: ReturnType<typeof createChartSeriesData>,
	) => createChartSeriesData(candles, { panes, shapes }, "1h", previous);

	it("places shapes in their panes, and the first line of a pane draws its levels", () => {
		const { panes, shapes } = series([
			line("sma", [], { placement: "overlay" }),
			line("rsi", []),
			line("rsi-sma", []),
			line("macd", [], { pane: "macd", placement: "pane" }),
		]);
		expect(
			shapes.map((shape) =>
				shape.kind === "line"
					? [shape.id, shape.pane, shape.levels.length]
					: undefined,
			),
		).toEqual([
			["sma", 0, 0],
			["rsi", 1, 1],
			["rsi-sma", 1, 0],
		]);
		expect(shapes[1]?.autoscaleInfoProvider).toBe(
			panes[0]?.autoscaleInfoProvider,
		);
	});

	it("keeps the autoscale of a pane while its scale does not change", () => {
		const first = series([]);
		expect(series([], [rsiPane], first).panes[0]).toBe(first.panes[0]);
		const rescaled = series(
			[],
			[{ ...rsiPane, scale: { ...rsiPane.scale, max: 90 } }],
			first,
		).panes[0];
		expect(rescaled?.autoscaleInfoProvider).not.toBe(
			first.panes[0]?.autoscaleInfoProvider,
		);
	});

	it("groups the legend by the indicators of its shapes", () => {
		const { shapes } = series([
			line("macd", [{ time: at(0), value: 1 }]),
			line("signal", [{ time: at(0), value: 2 }]),
			{ ...line("rsi", [{ time: at(0), value: 3 }]), indicator: "1" },
		]);
		expect(
			createIndicatorLegend({ candles: slots, shapes }, 0).map(
				({ indicator, items }) => [indicator, items.map(({ key }) => key)],
			),
		).toEqual([
			["0", ["macd", "signal"]],
			["1", ["rsi"]],
		]);
	});

	it("aligns a line with the slots and leaves out the joins before its breaks", () => {
		const [shape] = series([
			line("supertrend", [
				{ color: "green.5", time: at(0), value: 1 },
				{ color: "green.5", time: at(1), value: 2 },
				{ break: true, color: "red.5", time: at(2), value: 3 },
				{ time: at(4), value: 4 },
			]),
		]).shapes;
		if (shape?.kind !== "line") throw new Error("Missing line");
		expect(lineSeriesData(shape, color)).toEqual([
			{ color: "resolved green.5", time: times[0], value: 1 },
			{ color: "transparent", time: times[1], value: 2 },
			{ color: "resolved red.5", time: times[2], value: 3 },
			{ time: times[3] },
			{ time: times[4], value: 4 },
		]);
		// The legend shows the color of the point, not of its join.
		expect(
			times.map(
				(_, slot) =>
					createIndicatorLegend({ candles: slots, shapes: [shape] }, slot)[0]
						?.items[0],
			),
		).toEqual([
			{
				color: "green.5",
				key: "supertrend",
				title: "SUPERTREND",
				value: "1",
			},
			{
				color: "green.5",
				key: "supertrend",
				title: "SUPERTREND",
				value: "2",
			},
			{
				color: "red.5",
				key: "supertrend",
				title: "SUPERTREND",
				value: "3",
			},
			{
				color: "blue.5",
				key: "supertrend",
				title: "SUPERTREND",
				value: null,
			},
			{
				color: "blue.5",
				key: "supertrend",
				title: "SUPERTREND",
				value: "4",
			},
		]);
	});

	it("fills an area translucent in the color of each point", () => {
		const [shape] = series([
			{
				id: "fill",
				indicator: "0",
				kind: "area",
				placement: "overlay",
				points: [
					{ base: 5, color: "#00ff00", time: at(1), value: 8 },
					{ base: 5, color: "#ff0000", time: at(2), value: 9 },
				],
			},
		]).shapes;
		if (shape?.kind !== "area") throw new Error("Missing area");
		expect(areaFillPoints(shape.points, (token) => token)).toEqual([
			{ time: times[0] },
			{ base: 5, color: "rgba(0, 255, 0, 0.1)", time: times[1], value: 8 },
			{ base: 5, color: "rgba(255, 0, 0, 0.1)", time: times[2], value: 9 },
			{ time: times[3] },
			{ time: times[4] },
		]);
		expect(shape.values[1]).toEqual({ time: times[1], value: 8 });
	});

	describe("markers", () => {
		const markers = (points: ChartMarker[]): ChartMarkersShape => ({
			color: "yellow.5",
			id: "pivot",
			indicator: "0",
			kind: "markers",
			placement: "overlay",
			points,
			position: "above",
			title: "PIVOT",
		});
		const trade = {
			color: "green",
			position: "belowBar" as const,
			shape: "arrowUp" as const,
			time: times[1] as UTCTimestamp,
		};

		it("puts markers on their line or on the candles, only on candles the slots have", () => {
			const [shape] = series([
				markers([
					{ calculated_time: at(2), time: at(2), value: 12 },
					{ calculated_time: at(2), line_value: 50, time: at(0), value: 11 },
					// The slot of the missing candle has no marker.
					{ calculated_time: at(4), time: at(3), value: 13 },
					{ calculated_time: at(-1), time: at(-2), value: 9 },
				]),
			]).shapes;
			if (shape?.kind !== "markers") throw new Error("Missing markers");
			expect(lineMarkers(shape, color)).toEqual([
				{
					color: "resolved yellow.5",
					position: "aboveBar",
					shape: "circle",
					time: times[0],
				},
			]);
			expect(shape.onLine).toEqual([{ time: times[0], value: 50 }]);
			expect(
				createCandleMarkers([trade], [shape], color)?.map(({ shape, time }) => [
					shape,
					time,
				]),
			).toEqual([
				["arrowUp", times[1]],
				["circle", times[2]],
			]);
			expect(
				times.map(
					(_, slot) =>
						createIndicatorLegend({ candles: slots, shapes: [shape] }, slot)[0]
							?.items[0]?.value,
				),
			).toEqual(["11", null, "12", null, null]);
		});

		it("keeps the markers of the source without markers on the candles", () => {
			const shapes = series([markers([])]).shapes;
			expect(createCandleMarkers([trade], shapes, color)).toEqual([trade]);
		});
	});

	describe("segments", () => {
		const segments = (
			items: [from: number, to: number][],
		): ChartSegmentsShape => ({
			color: "green.5",
			id: "bull-candles",
			indicator: "0",
			kind: "segments",
			placement: "overlay",
			segments: items.map(([from, to]) => ({
				calculated_time: at(to),
				from: { time: at(from), value: from },
				to: { time: at(to), value: to },
			})),
		});
		const data = (items: [from: number, to: number][]) => {
			const [shape] = series([segments(items)]).shapes;
			return shape?.kind === "segments" ? shape.data : undefined;
		};

		it("joins segments sharing a point and leaves out the gaps between others", () => {
			expect(
				data([
					[0, 1],
					[1, 2],
					[2, 4],
				]),
			).toEqual([
				{ time: times[0], value: 0 },
				{ time: times[1], value: 1 },
				{ time: times[2], value: 2 },
				{ time: times[4], value: 4 },
			]);
			expect(
				data([
					[0, 1],
					[2, 4],
				]),
			).toEqual([
				{ time: times[0], value: 0 },
				{ color: "transparent", time: times[1], value: 1 },
				{ time: times[2], value: 2 },
				{ time: times[4], value: 4 },
			]);
		});

		it("does not draw a segment starting before the end of the one before, or off the slots", () => {
			expect(
				data([
					[1, 4],
					[2, 4],
					[-1, 0],
					[2, 3],
				]),
			).toEqual([
				{ time: times[1], value: 1 },
				{ time: times[4], value: 4 },
			]);
		});
	});
});

describe("fitScaleToLevels", () => {
	const rsi = {
		levels: [
			{ title: "Oversold", value: 30 },
			{ title: "Overbought", value: 70 },
		],
		max: 100,
		min: 0,
	};

	it("keeps the levels in view when values stay between them", () => {
		expect(
			fitScaleToLevels({ priceRange: { maxValue: 57, minValue: 40 } }, rsi)
				?.priceRange,
		).toEqual({ maxValue: 70, minValue: 30 });
	});

	it("extends past the levels to fit the values", () => {
		expect(
			fitScaleToLevels({ priceRange: { maxValue: 85, minValue: 22 } }, rsi)
				?.priceRange,
		).toEqual({ maxValue: 85, minValue: 22 });
	});

	it("shows only the levels without visible values", () => {
		expect(fitScaleToLevels(null, rsi)?.priceRange).toEqual({
			maxValue: 70,
			minValue: 30,
		});
	});

	it("keeps the visible range of an unbounded indicator", () => {
		const visible = { priceRange: { maxValue: 2, minValue: -1 } };
		expect(fitScaleToLevels(visible, { levels: [] })).toEqual(visible);
	});
});

describe("visible candles", () => {
	const at = (time: number) => time as UTCTimestamp;
	const slots = [
		{ close: 5, high: 9, low: 1, open: 5, time: at(1) },
		{ close: 5, high: 12, low: 4, open: 5, time: at(2) },
		{ time: at(3) },
		{ close: 5, high: 8, low: 0.5, open: 5, time: at(4) },
	];
	const options = {
		max: { color: "max", position: "aboveBar", shape: "arrowDown" },
		min: { color: "min", position: "belowBar", shape: "arrowUp" },
	} as const;

	it("marks the extremes of the visible candles on their candles, merged with the other markers", () => {
		const minMax = getVisibleMinMax(slots, { from: 1, to: 3 });
		expect(minMax).toEqual({
			max: { price: 12, time: at(2) },
			min: { price: 0.5, time: at(4) },
		});
		if (minMax === null) throw new Error("Missing min and max");
		const trade = {
			color: "green",
			position: "belowBar" as const,
			shape: "arrowUp" as const,
			time: at(2),
		};
		expect(
			withMinMaxMarkers([trade], minMax, options, 2).map(
				({ color, text, time }) => [color, text, time],
			),
		).toEqual([
			["max", "12", at(2)],
			["green", undefined, at(2)],
			["min", "0.5", at(4)],
		]);
	});

	it("finds the rightmost visible candle, skipping missing candles", () => {
		expect(lastVisibleCandleIndex(slots, { from: 0, to: 2.6 })).toBe(1);
		expect(lastVisibleCandleIndex(slots, { from: 0, to: 9 })).toBe(3);
		expect(lastVisibleCandleIndex(slots, null)).toBe(3);
	});

	it("has no extremes without visible candles", () => {
		expect(getVisibleMinMax(slots, { from: 2, to: 2 })).toBeNull();
		expect(getVisibleMinMax(slots, null)).toBeNull();
	});
});
