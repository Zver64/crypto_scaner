import { describe, expect, it, vi } from "vitest";
import {
	createCandlestickData,
	createIndicatorData,
	createMarkerDataSelector,
	fitPaneIndicatorScale,
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

describe("fitPaneIndicatorScale", () => {
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
			fitPaneIndicatorScale({ priceRange: { maxValue: 57, minValue: 40 } }, rsi)
				?.priceRange,
		).toEqual({ maxValue: 70, minValue: 30 });
	});

	it("extends past the levels to fit the values", () => {
		expect(
			fitPaneIndicatorScale({ priceRange: { maxValue: 85, minValue: 22 } }, rsi)
				?.priceRange,
		).toEqual({ maxValue: 85, minValue: 22 });
	});

	it("shows only the levels without visible values", () => {
		expect(fitPaneIndicatorScale(null, rsi)?.priceRange).toEqual({
			maxValue: 70,
			minValue: 30,
		});
	});

	it("keeps the visible range of an unbounded indicator", () => {
		const visible = { priceRange: { maxValue: 2, minValue: -1 } };
		expect(fitPaneIndicatorScale(visible, { levels: [] })).toEqual(visible);
	});
});
