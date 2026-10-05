import { describe, expect, it } from "vitest";
import { parseStrategyBacktestSearch } from "@/routes/-strategy-backtest-search";

describe("parseStrategyBacktestSearch", () => {
	it("keeps the strategy id and the coin list state, dropping the rest", () => {
		expect(
			parseStrategyBacktestSearch({
				period: 30,
				sort_column: "symbol",
				sort_direction: "asc",
				strategy: 4,
				symbol_filter: "BTC",
			}),
		).toEqual({
			sort_column: "symbol",
			sort_direction: "asc",
			strategy: 4,
			symbol_filter: "BTC",
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
});
