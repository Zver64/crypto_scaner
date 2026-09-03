import { describe, expect, it } from "vitest";
import type {
	MarketScanItem,
	UnresolvedInstrumentCode,
} from "../../api/client";
import {
	binanceSpotUrl,
	filterMarketScanItems,
	formatRangePercent,
	marketCapUnavailableReason,
	sortMarketScanItems,
} from "./results";

const items: MarketScanItem[] = [
	{ evaluations: [], matched: true, symbol: "ZZZUSDT" },
	{ evaluations: [], matched: true, symbol: "AdaUsdt" },
	{ evaluations: [], matched: true, symbol: "AAAUSDT" },
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

describe("Market Cap presentation", () => {
	it.each<[UnresolvedInstrumentCode, string]>([
		[
			"mapping_not_found",
			"No market capitalization match was found for this instrument.",
		],
		[
			"mapping_conflict",
			"Multiple market capitalization matches were found for this instrument.",
		],
		[
			"mapping_provider_unavailable",
			"Market capitalization mapping data is temporarily unavailable.",
		],
		[
			"market_cap_missing",
			"Market capitalization data is unavailable for this instrument.",
		],
	])("explains unresolved code %s", (code, expected) => {
		expect(marketCapUnavailableReason(code)).toBe(expected);
	});
});

describe("binanceSpotUrl", () => {
	it("builds a Binance Spot trading URL for a USDT symbol", () => {
		expect(binanceSpotUrl("BTCUSDT")).toBe(
			"https://www.binance.com/en/trade/BTC_USDT?type=spot",
		);
	});

	it("normalizes surrounding whitespace and symbol casing", () => {
		expect(binanceSpotUrl(" adausdt ")).toBe(
			"https://www.binance.com/en/trade/ADA_USDT?type=spot",
		);
	});

	it.each(["", "USDT", "BTCEUR"])("rejects unsupported symbol %j", (symbol) => {
		expect(binanceSpotUrl(symbol)).toBeUndefined();
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
			evaluations: [],
			matched: true,
			symbol: "AAAUSDT",
		});
	});
});

describe("sortMarketScanItems", () => {
	const daily = {
		candle_count: 30,
		from: "2026-08-01T00:00:00Z",
		key: "daily_volatility",
		label: "Daily Volatility",
		matched: true,
		metrics: { range_percent: 4 },
		name: "volatility",
		to: "2026-08-02T00:00:00Z",
	};
	const hourly = {
		...daily,
		candle_count: 60,
		key: "hourly_volatility",
		label: "Hourly Volatility",
		metrics: { range_percent: 2 },
	};
	const marketCap = {
		...daily,
		candle_count: 0,
		key: "market_cap",
		label: "Market Cap",
		metrics: { market_cap_usd: 400 },
		name: "market_cap",
	};
	const sortableItems = [
		{
			evaluations: [daily, hourly, marketCap],
			matched: true,
			symbol: "ZEBRAUSDT",
		},
		{
			evaluations: [
				{ ...daily, metrics: { range_percent: 6 } },
				{ ...hourly, metrics: { range_percent: 1 } },
				{ ...marketCap, metrics: { market_cap_usd: 900 } },
			],
			matched: true,
			symbol: "ALPHAUSDT",
		},
		{
			evaluations: [
				{ ...daily, metrics: { range_percent: 6 } },
				{ ...hourly, metrics: { range_percent: 3 } },
				{ ...marketCap, metrics: { market_cap_usd: 100 } },
			],
			matched: true,
			symbol: "BRAVOUSDT",
		},
		{
			evaluations: [
				{ ...daily, metrics: { range_percent: 5 } },
				{ ...hourly, metrics: { range_percent: 3 } },
				{ ...marketCap, metrics: { market_cap_usd: 100 } },
			],
			matched: true,
			symbol: "CHARLIEUSDT",
		},
	];

	it.each([
		[
			"daily_volatility",
			"desc",
			["ALPHAUSDT", "BRAVOUSDT", "CHARLIEUSDT", "ZEBRAUSDT"],
		],
		[
			"daily_volatility",
			"asc",
			["ZEBRAUSDT", "CHARLIEUSDT", "ALPHAUSDT", "BRAVOUSDT"],
		],
		[
			"hourly_volatility",
			"desc",
			["BRAVOUSDT", "CHARLIEUSDT", "ZEBRAUSDT", "ALPHAUSDT"],
		],
		[
			"hourly_volatility",
			"asc",
			["ALPHAUSDT", "ZEBRAUSDT", "BRAVOUSDT", "CHARLIEUSDT"],
		],
		[
			"market_cap",
			"desc",
			["ALPHAUSDT", "ZEBRAUSDT", "BRAVOUSDT", "CHARLIEUSDT"],
		],
		[
			"market_cap",
			"asc",
			["BRAVOUSDT", "CHARLIEUSDT", "ZEBRAUSDT", "ALPHAUSDT"],
		],
	] as const)("sorts %s %s with alphabetical ties", (column, direction, symbols) => {
		expect(
			sortMarketScanItems(sortableItems, { column, direction }).map(
				({ symbol }) => symbol,
			),
		).toEqual(symbols);
	});
});
