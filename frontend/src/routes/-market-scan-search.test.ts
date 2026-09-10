import { describe, expect, it } from "vitest";
import { defaultMarketScanSort } from "@/features/market-scan/sort";
import {
	marketScanSortFromSearch,
	parseMarketScanSearch,
} from "@/routes/-market-scan-search";

describe("parseMarketScanSearch", () => {
	it("keeps valid table filter and sort state", () => {
		const search = parseMarketScanSearch({
			sort_column: "dailyRangePercent",
			sort_direction: "asc",
			symbol_filter: "BTC",
		});

		expect(search).toEqual({
			sort_column: "dailyRangePercent",
			sort_direction: "asc",
			symbol_filter: "BTC",
		});
		expect(marketScanSortFromSearch(search)).toEqual({
			column: "dailyRangePercent",
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

	it("drops invalid table state and uses the default sort", () => {
		const search = parseMarketScanSearch({
			sort_column: "symbol",
			sort_direction: "sideways",
			symbol_filter: 42,
		});

		expect(search).toEqual({});
		expect(marketScanSortFromSearch(search)).toEqual(defaultMarketScanSort);
	});
});
