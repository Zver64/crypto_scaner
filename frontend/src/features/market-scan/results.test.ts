import { describe, expect, it } from "vitest";
import type { MarketScanItem } from "../../api/client";
import { filterMarketScanItems, formatRangePercent } from "./results";

const items: MarketScanItem[] = [
	{ candle_count: 30, range_percent: 9.4381, symbol: "ZZZUSDT" },
	{ candle_count: 29, range_percent: 1, symbol: "AdaUsdt" },
	{ candle_count: 28, range_percent: 0.004567, symbol: "AAAUSDT" },
];

describe("formatRangePercent", () => {
	it.each([
		[0, "0%"],
		[0.004567, "0.00457%"],
		[0.9995, "1%"],
		[1.005, "1.01%"],
		[9.4381, "9.44%"],
		[1234.5, "1,230%"],
	])("formats %s with three significant digits as %s", (value, expected) => {
		expect(formatRangePercent(value)).toBe(expected);
	});
});

describe("filterMarketScanItems", () => {
	it("returns the complete backend-ordered result for an empty filter", () => {
		expect(filterMarketScanItems(items, "  ")).toEqual(items);
	});

	it("matches partial symbols without case sensitivity", () => {
		expect(filterMarketScanItems(items, "usdt")).toEqual(items);
		expect(filterMarketScanItems(items, "ada")).toEqual([items[1]]);
	});

	it("returns no rows for a missing symbol", () => {
		expect(filterMarketScanItems(items, "BTC")).toEqual([]);
	});

	it("preserves exact symbols, values, object identity, and backend order", () => {
		const filtered = filterMarketScanItems(items, "a");

		expect(filtered).toEqual([items[1], items[2]]);
		expect(filtered[0]).toBe(items[1]);
		expect(items[2]).toEqual({
			candle_count: 28,
			range_percent: 0.004567,
			symbol: "AAAUSDT",
		});
	});
});
