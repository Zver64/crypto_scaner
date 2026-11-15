import { describe, expect, it } from "vitest";

import type {
	ChartMarkersShape,
	ChartPageResponse,
	ChartSegmentsShape,
	ChartShape,
} from "@/api/generated/models";
import {
	mergeChartTail,
	validateChartPage,
} from "@/features/instrument-analysis/chart-page";

function candle(hour: number) {
	const open = new Date(Date.UTC(2026, 7, 27, hour));
	return {
		close: 12,
		close_time: new Date(open.getTime() + 3_599_999).toISOString(),
		high: 13,
		low: 9,
		open: 10,
		open_time: open.toISOString().replace(".000Z", "Z"),
		quote_asset_volume: 20,
		trade_count: 4,
		volume: 10,
	};
}
const time = (hour: number) => candle(hour).open_time;

function rsi(points: { hour: number; value: number }[]): ChartShape {
	return {
		color: "blue.5",
		id: "rsi",
		kind: "line",
		pane: "rsi",
		placement: "pane",
		points: points.map(({ hour, value }) => ({ time: time(hour), value })),
		title: "RSI 14",
	};
}

function response(
	overrides: Partial<ChartPageResponse> = {},
): ChartPageResponse {
	return {
		candles: [candle(0)],
		has_more: true,
		interval: "1h",
		next_before: time(0),
		panes: [{ key: "rsi", scale: { levels: [], max: 100, min: 0 } }],
		shapes: [rsi([{ hour: 0, value: 62.5 }])],
		symbol: "BTCUSDT",
		...overrides,
	};
}

describe("chart page contract", () => {
	it("accepts shapes on the candles of the page", () => {
		const page = response({
			candles: [candle(0), candle(1)],
			shapes: [
				rsi([
					{ hour: 0, value: 55 },
					{ hour: 1, value: 70 },
				]),
				{
					color: "yellow.5",
					id: "pivot",
					kind: "markers",
					placement: "overlay",
					points: [{ calculated_time: time(1), time: time(0), value: 13 }],
					position: "above",
					title: "PIVOT",
				},
			],
		});
		expect(validateChartPage(page, "BTCUSDT", "1h")).toBe(page);
	});

	it.each([
		{ shapes: [rsi([{ hour: 1, value: 62.5 }])] },
		{ shapes: [rsi([]), rsi([])] },
		{ panes: [] },
		{ shapes: [{ ...rsi([]), pane: "macd" }] },
		{ shapes: [{ ...rsi([]), placement: "overlay" as const }] },
		{
			candles: [candle(0), candle(1)],
			shapes: [
				rsi([
					{ hour: 1, value: 55 },
					{ hour: 0, value: 70 },
				]),
			],
		},
	])("rejects shapes the page cannot draw", (overrides) => {
		expect(() =>
			validateChartPage(response(overrides), "BTCUSDT", "1h"),
		).toThrow("An unexpected error occurred. Please try again.");
	});
});

describe("chart tail", () => {
	const bull: ChartMarkersShape = {
		color: "green.5",
		id: "bull",
		kind: "markers",
		pane: "rsi",
		placement: "pane",
		points: [
			{ calculated_time: time(1), line_value: 50, time: time(0), value: 1 },
			{ calculated_time: time(2), line_value: 60, time: time(1), value: 2 },
		],
		position: "below",
		title: "Bull",
	};
	const bullCandles: ChartSegmentsShape = {
		color: "green.5",
		id: "bull-candles",
		kind: "segments",
		placement: "overlay",
		segments: [
			{
				calculated_time: time(2),
				from: { time: time(0), value: 9 },
				to: { time: time(1), value: 9 },
			},
		],
	};
	const base = response({
		candles: [candle(0), candle(1), candle(2)],
		has_more: false,
		next_before: null,
		shapes: [
			rsi([
				{ hour: 0, value: 50 },
				{ hour: 1, value: 60 },
				{ hour: 2, value: 70 },
			]),
			bull,
			bullCandles,
		],
	});
	const tail = (shapes: ChartShape[]) =>
		response({
			candles: [{ ...candle(2), close: 15 }],
			has_more: false,
			next_before: null,
			shapes,
		});

	it("replaces what the current candle calculated, wherever it is drawn", () => {
		const merged = mergeChartTail(
			base,
			tail([
				rsi([{ hour: 2, value: 72 }]),
				{ ...bull, points: [] },
				{ ...bullCandles, segments: [] },
			]),
		);
		expect(merged.candles.map(({ close }) => close)).toEqual([12, 12, 15]);
		expect(merged.shapes).toEqual([
			rsi([
				{ hour: 0, value: 50 },
				{ hour: 1, value: 60 },
				{ hour: 2, value: 72 },
			]),
			{ ...bull, points: bull.points.slice(0, 1) },
			{ ...bullCandles, segments: [] },
		]);
	});

	it("rejects a tail with other shapes", () => {
		expect(() =>
			mergeChartTail(
				base,
				tail([rsi([]), { ...bull, id: "bear" }, bullCandles]),
			),
		).toThrow("An unexpected error occurred. Please try again.");
	});
});
