import { describe, expect, it } from "vitest";
import {
	defaultMarketScanSort,
	nextMarketScanSort,
} from "@/features/market-scan/sort";

describe("Market Scan sorting", () => {
	it("starts a new column descending and toggles the active column", () => {
		expect(
			nextMarketScanSort(defaultMarketScanSort, "hourly_volatility"),
		).toEqual({
			column: "hourly_volatility",
			direction: "desc",
		});
		expect(
			nextMarketScanSort(defaultMarketScanSort, "daily_volatility"),
		).toEqual({
			column: "daily_volatility",
			direction: "asc",
		});
	});
});
