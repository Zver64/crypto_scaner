import { describe, expect, it } from "vitest";
import type { listInstrumentCandlesResponseSuccess } from "@/api/generated/api";
import {
	nextCandlePageParam,
	validateCandlePage,
} from "@/features/instrument-analysis/candle-page";

const candle = {
	open_time: "2026-08-01T00:00:00Z",
	close_time: "2026-08-01T00:59:59.999Z",
	open: 1,
	high: 3,
	low: 0.5,
	close: 2,
	volume: 10,
	quote_asset_volume: 20,
	trade_count: 4,
};

function response(
	overrides: Partial<
		Extract<listInstrumentCandlesResponseSuccess, { status: 200 }>["data"]
	> = {},
): listInstrumentCandlesResponseSuccess {
	return {
		data: {
			candles: [candle],
			has_more: true,
			interval: "1h",
			next_before: candle.open_time,
			symbol: "BTCUSDT",
			...overrides,
		},
		headers: new Headers(),
		status: 200,
	};
}

describe("candle page semantics", () => {
	it("accepts a chronological page and exposes its generated cursor", () => {
		const page = response();
		expect(validateCandlePage(page, "BTCUSDT", "1h").candles).toEqual([candle]);
		expect(nextCandlePageParam(page)).toBe(candle.open_time);
	});

	it.each([
		{ interval: "1d" as const },
		{ candles: [{ ...candle, high: 1 }] },
		{ has_more: false, next_before: candle.open_time },
		{ candles: [{ ...candle, open_time: "invalid" }] },
	])("rejects invalid business invariants %#", (overrides) => {
		expect(() =>
			validateCandlePage(response(overrides), "BTCUSDT", "1h"),
		).toThrow("An unexpected error occurred. Please try again.");
	});

	it("stops infinite pagination when the generated response has no cursor", () => {
		expect(
			nextCandlePageParam(
				response({ candles: [], has_more: false, next_before: undefined }),
			),
		).toBeUndefined();
	});
});
