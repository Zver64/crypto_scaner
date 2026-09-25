import { describe, expect, it } from "vitest";
import type { ChartPageResponse } from "@/api/generated/models";
import {
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
	overrides: Partial<ChartPageResponse> = {},
): ChartPageResponse {
	return {
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
	};
}

describe("chart page contract", () => {
	it("validates synchronized candles and RSI", () => {
		const page = response();
		const validated = validateChartPage(page, "BTCUSDT", "1h");
		expect(rsiPoints(validated)).toEqual([
			{ time: candle.open_time, value: 62.5 },
		]);
	});

	it("accepts a replacement range with revised values across the whole history", () => {
		const older = {
			...candle,
			open_time: "2026-08-26T23:00:00Z",
			close_time: "2026-08-26T23:59:59.999Z",
		};
		const range = response({
			candles: [older, candle],
			indicators: [
				{
					type: "rsi",
					parameters: { period: 14 },
					series: [
						{
							name: "rsi",
							points: [
								{ time: older.open_time, value: 55 },
								{ time: candle.open_time, value: 70 },
							],
						},
					],
				},
			],
			next_before: older.open_time,
		});
		expect(rsiPoints(validateChartPage(range, "BTCUSDT", "1h"))).toEqual([
			{ time: older.open_time, value: 55 },
			{ time: candle.open_time, value: 70 },
		]);
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
