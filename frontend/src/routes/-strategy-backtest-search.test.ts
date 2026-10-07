import { describe, expect, it } from "vitest";
import { parseStrategyBacktestSearch } from "@/routes/-strategy-backtest-search";

describe("parseStrategyBacktestSearch", () => {
	it("keeps the strategy, normalized coin, favorites filter and hold, dropping old list state", () => {
		expect(
			parseStrategyBacktestSearch({
				all_coins: true,
				hold: 24,
				period: 30,
				sort_column: "symbol",
				sort_direction: "asc",
				strategy: 4,
				symbol: " btcusdt ",
				symbol_filter: "BTC",
			}),
		).toEqual({
			all_coins: true,
			hold: 24,
			strategy: 4,
			symbol: "BTCUSDT",
		});
	});

	it.each([
		"4",
		0,
		-1,
		1.5,
		null,
	])("drops the invalid strategy id %s", (strategy) => {
		expect(parseStrategyBacktestSearch({ strategy })).toEqual({});
	});

	it.each([
		"",
		"   ",
		123,
		null,
		["BTCUSDT"],
	])("drops the invalid coin %s", (symbol) => {
		expect(parseStrategyBacktestSearch({ symbol })).toEqual({});
	});

	it.each(["true", false, 1])("drops the invalid all_coins %s", (allCoins) => {
		expect(parseStrategyBacktestSearch({ all_coins: allCoins })).toEqual({});
	});

	it.each([1, 1000])("keeps the hold %s at the limits", (hold) => {
		expect(parseStrategyBacktestSearch({ hold })).toEqual({ hold });
	});

	it.each(["24", 0, 1001, 2.5, null])("drops the invalid hold %s", (hold) => {
		expect(parseStrategyBacktestSearch({ hold })).toEqual({});
	});
});
