import { describe, expect, it } from "vitest";
import {
	marketScanSortFromSearch,
	parseMarketScanSearch,
	parseMarketScanSortSearch,
} from "@/routes/-market-scan-search";

describe("parseMarketScanSortSearch", () => {
	it("keeps valid sort state and drops unrelated search parameters", () => {
		expect(
			parseMarketScanSortSearch({
				sort_column: "market_cap_usd",
				sort_direction: "asc",
				symbol_filter: "BTC",
			}),
		).toEqual({
			sort_column: "market_cap_usd",
			sort_direction: "asc",
		});
	});

	it("drops incomplete sort state", () => {
		expect(
			parseMarketScanSortSearch({ sort_column: "market_cap_usd" }),
		).toEqual({});
	});
});

describe("parseMarketScanSearch", () => {
	it("keeps valid table filter and sort state", () => {
		const search = parseMarketScanSearch({
			sort_column: "daily_range_percent",
			sort_direction: "asc",
			symbol_filter: "BTC",
		});

		expect(search).toEqual({
			sort_column: "daily_range_percent",
			sort_direction: "asc",
			symbol_filter: "BTC",
		});
		expect(marketScanSortFromSearch(search)).toEqual({
			column: "daily_range_percent",
			direction: "asc",
		});
	});

	it("drops invalid form criteria", () => {
		expect(
			parseMarketScanSearch({
				hourly_minimum_range_percent: "",
				period: 999_999,
			}),
		).toEqual({});
	});

	it("drops invalid table state and leaves the sort to the table default", () => {
		const search = parseMarketScanSearch({
			sort_column: "symbol",
			sort_direction: "sideways",
			symbol_filter: 42,
		});

		expect(search).toEqual({});
		expect(marketScanSortFromSearch(search)).toBeUndefined();
	});
});
