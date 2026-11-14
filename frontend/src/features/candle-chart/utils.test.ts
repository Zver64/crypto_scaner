import { describe, expect, it, vi } from "vitest";

import {
	createAnchoredMarkers,
	createCandleMarkers,
	createCandlestickData,
	createIndicatorLegend,
	createIndicatorData,
	createIndicatorPanes,
	createMarkerDataSelector,
	createMarkerSegments,
	fitScaleToLevels,
	formatCandleRange,
	formatChartTime,
	formatOhlc,
	formatPrice,
	nextCandleOpen,
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

describe("candle markers", () => {
	const slots = createCandlestickData(
		[0, 1, 2, 3].map((slot) => candle(slot, 10)),
		"1h",
	);
	const times = slots.map(({ time }) => time);
	const lines = {
		"pivot:high": {
			data: createIndicatorData(slots, [
				{ time: candle(1, 0).open_time, value: 11 },
				{ time: candle(3, 0).open_time, value: 12 },
			]),
			resolution: { base: 100, fractionDigits: 2, minMove: 0.01 },
		},
	};
	const pivot = (shift: number) => [
		{
			id: "pivot",
			placement: "overlay" as const,
			lines: [
				{
					color: "yellow.5",
					output: "high",
					shift,
					style: "marker_above" as const,
					title: "PIVOT high",
				},
			],
		},
	];
	const color = (token: string) => `resolved ${token}`;

	it("draws marker lines shifted back and drops those before the first slot", () => {
		expect(createCandleMarkers(undefined, pivot(2), lines, color)).toEqual([
			{
				color: "resolved yellow.5",
				position: "aboveBar",
				shape: "circle",
				time: times[1],
			},
		]);
	});

	it("shows a marker line in the legend on the candle its marker is drawn on", () => {
		expect(
			times.map(
				(_, slot) => createIndicatorLegend(pivot(2), lines, slot)[0]?.value,
			),
		).toEqual([null, "12", null, null]);
	});

	it("merges them with the markers of the source in time order", () => {
		const trade = {
			color: "green",
			position: "belowBar" as const,
			shape: "arrowUp" as const,
			time: times[2],
		};
		expect(
			createCandleMarkers([trade], pivot(0), lines, color)?.map(
				({ shape, time }) => [shape, time],
			),
		).toEqual([
			["circle", times[1]],
			["arrowUp", times[2]],
			["circle", times[3]],
		]);
	});

	it("keeps the markers of the source without marker lines", () => {
		const markers = [
			{
				color: "green",
				position: "belowBar" as const,
				shape: "arrowUp" as const,
				time: times[0],
			},
		];
		expect(createCandleMarkers(markers, [], lines, color)).toBe(markers);
	});

	it("puts the markers of anchored lines on the line of their anchor, where it has values", () => {
		const anchored = [
			{
				...pivot(0)[0],
				lines: pivot(0)[0].lines.map((line) => ({
					...line,
					anchor: { indicator_id: "rsi", output: "rsi" },
				})),
			},
		];
		const withAnchor = {
			...lines,
			// The first pivot lies before the first value of its anchor.
			"rsi:rsi": {
				...lines["pivot:high"],
				data: createIndicatorData(slots, [
					{ time: candle(2, 0).open_time, value: 50 },
					{ time: candle(3, 0).open_time, value: 60 },
				]),
			},
		};
		expect(createCandleMarkers(undefined, anchored, withAnchor, color)).toBe(
			undefined,
		);
		expect(createAnchoredMarkers(anchored, withAnchor, color)).toEqual({
			"rsi:rsi": [
				{
					color: "resolved yellow.5",
					position: "aboveBar",
					shape: "circle",
					time: times[3],
				},
			],
		});
	});
});

describe("marker segments", () => {
	const candles = [0, 1, 2, 3, 4, 5, 6].map((slot) => candle(slot, 10 + slot));
	const slots = createCandlestickData(candles, "1h");
	const times = slots.map(({ time }) => time);
	const open = (slot: number) => candles[slot].open_time;
	const resolution = { base: 100, fractionDigits: 2, minMove: 0.01 };
	// Values on slots 1, 2, 4 and 6, whose markers are drawn one slot earlier.
	const series = {
		candles: slots,
		lines: {
			"pivot:low": {
				data: createIndicatorData(slots, [
					{ time: open(1), value: 20 },
					{ time: open(2), value: 30 },
					{ time: open(4), value: 40 },
					{ time: open(6), value: 35 },
				]),
				resolution,
			},
		},
	};
	const layout = (
		style: "marker_above" | "marker_below",
		anchor?: { indicator_id: string; output: string },
	) => {
		const indicators = [
			{
				id: "macd",
				placement: "pane" as const,
				pane: "macd",
				lines: [],
				scale: { levels: [] },
			},
			{
				id: "rsi",
				placement: "pane" as const,
				pane: "rsi",
				lines: [],
				scale: { levels: [] },
			},
			{
				id: "pivot",
				placement: "overlay" as const,
				lines: [
					{
						anchor,
						color: "yellow.5",
						output: "low",
						shift: 1,
						style,
						title: "PIVOT low",
					},
				],
			},
		];
		return { indicators, panes: createIndicatorPanes(indicators) };
	};

	it("connects the value under a connected candle with the previous one where their markers are drawn", () => {
		expect(
			createMarkerSegments([open(6)], layout("marker_below"), series, "1h"),
		).toEqual([
			{
				color: "yellow.5",
				data: [
					{ time: times[3], value: 40 },
					{ time: times[5], value: 35 },
				],
				key: "pivot:low",
				pane: 0,
				scale: "price",
			},
		]);
	});

	it("draws the segments of an anchored line in the pane of its anchor and between the same candles", () => {
		const anchor = { indicator_id: "rsi", output: "rsi" };
		expect(
			createMarkerSegments(
				[open(4), open(6)],
				layout("marker_below", anchor),
				series,
				"1h",
			),
		).toEqual([
			{
				color: "yellow.5",
				data: [
					{ time: times[1], value: 30 },
					{ time: times[3], value: 40 },
					{ time: times[5], value: 35 },
				],
				key: "pivot:low",
				pane: 2,
				scale: "price",
			},
			{
				color: "yellow.5",
				data: [
					{ time: times[1], value: 10 },
					{ time: times[3], value: 12 },
					{ time: times[5], value: 14 },
				],
				key: "pivot:low:candles",
				pane: 0,
				scale: "price",
			},
		]);
		expect(
			createMarkerSegments(
				[open(6)],
				layout("marker_above", anchor),
				series,
				"1h",
			)[1]?.data,
		).toEqual([
			{ time: times[3], value: 14 },
			{ time: times[5], value: 16 },
		]);
	});

	it("does not join separate segments", () => {
		expect(
			createMarkerSegments(
				[open(2), open(6)],
				layout("marker_below"),
				series,
				"1h",
			)[0]?.data,
		).toEqual([
			{ time: times[0], value: 20 },
			{ color: "transparent", time: times[1], value: 30 },
			{ time: times[3], value: 40 },
			{ time: times[5], value: 35 },
		]);
	});

	it("draws nothing for the first value of a line, a candle without a value, or no connected candles", () => {
		expect(
			createMarkerSegments(
				[open(1), open(5)],
				layout("marker_below"),
				series,
				"1h",
			),
		).toEqual([]);
		expect(
			createMarkerSegments(undefined, layout("marker_below"), series, "1h"),
		).toEqual([]);
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
