import { describe, expect, it } from "vitest";
import type { getInstrumentChartResponseSuccess } from "@/api/generated/api";
import {
	mergeChartCandlePages,
	mergeChartRsiPages,
	nextChartPageParam,
	rsiPoints,
	validateChartPage,
} from "@/features/instrument-analysis/chart-page";

const candle = {
	close: 12,
	close_time: "2026-08-27T00:59:59.999Z",
	high: 13,
	low: 9,
	open: 10,
	open_time: "2026-08-27T00:00:00Z",
	quote_asset_volume: 20,
	trade_count: 4,
	volume: 10,
};

function response(
	overrides: Partial<getInstrumentChartResponseSuccess["data"]> = {},
): getInstrumentChartResponseSuccess {
	return {
		data: {
			candles: [candle],
			has_more: true,
			indicators: [
				{
					parameters: { period: 14 },
					series: [
						{
							name: "rsi",
							points: [{ time: candle.open_time, value: 62.5 }],
						},
					],
					type: "rsi",
				},
			],
			interval: "1h",
			next_before: candle.open_time,
			symbol: "BTCUSDT",
			...overrides,
		},
		headers: new Headers(),
		status: 200,
	};
}

describe("chart page contract", () => {
	it("validates synchronized candles and RSI and advances by the visible cursor", () => {
		const page = response();
		const validated = validateChartPage(page, "BTCUSDT", "1h");
		expect(rsiPoints(validated)).toEqual([
			{ time: candle.open_time, value: 62.5 },
		]);
		expect(nextChartPageParam(page)).toBe(candle.open_time);
	});

	it("refreshes only the latest overlay without changing loaded pages or cursors", () => {
		const latest = response().data;
		const olderCandle = {
			...candle,
			open_time: "2026-08-26T23:00:00Z",
			close_time: "2026-08-26T23:59:59.999Z",
			close: 11,
		};
		const older = response({
			candles: [olderCandle],
			next_before: olderCandle.open_time,
			indicators: [
				{
					parameters: { period: 14 },
					series: [
						{
							name: "rsi",
							points: [{ time: olderCandle.open_time, value: 55 }],
						},
					],
					type: "rsi",
				},
			],
		}).data;
		const refreshed = response({
			candles: [{ ...candle, close: 15 }],
			indicators: [
				{
					parameters: { period: 14 },
					series: [
						{
							name: "rsi",
							points: [{ time: candle.open_time, value: 70 }],
						},
					],
					type: "rsi",
				},
			],
		}).data;
		const pages = [latest, older];
		const pageParams = [undefined, latest.next_before];

		expect(
			mergeChartCandlePages(pages, refreshed).map((item) => item.close),
		).toEqual([11, 15]);
		expect(
			mergeChartRsiPages(pages, refreshed).map((item) => item.value),
		).toEqual([55, 70]);
		expect(pages).toEqual([latest, older]);
		expect(pageParams).toEqual([undefined, candle.open_time]);
	});

	it.each([
		{ indicators: [] },
		{
			indicators: [
				{
					parameters: { period: 7 },
					series: [{ name: "rsi", points: [] }],
					type: "rsi",
				},
			],
		},
		{
			indicators: [
				{
					parameters: { period: 14 },
					series: [
						{
							name: "rsi",
							points: [{ time: "2026-08-26T23:00:00Z", value: 62.5 }],
						},
					],
					type: "rsi",
				},
			],
		},
		{
			indicators: [
				{
					parameters: { period: 14 },
					series: [
						{
							name: "rsi",
							points: [{ time: candle.open_time, value: 101 }],
						},
					],
					type: "rsi",
				},
			],
		},
	])("rejects a desynchronized chart response", (overrides) => {
		expect(() =>
			validateChartPage(response(overrides), "BTCUSDT", "1h"),
		).toThrow();
	});
});
