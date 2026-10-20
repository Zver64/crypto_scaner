import { describe, expect, it } from "vitest";

import { parseStrategyBacktestSearch } from "@/routes/-strategy-backtest-search";

describe("parseStrategyBacktestSearch", () => {
	it("normalizes multiple coins and gives them precedence over a legacy coin", () => {
		expect(
			parseStrategyBacktestSearch({
				symbol: "ARBUSDT",
				symbols: [" btcusdt ", "ETHUSDT", "BTCUSDT", null, ""],
			}),
		).toEqual({ symbols: ["BTCUSDT", "ETHUSDT"] });
		expect(
			parseStrategyBacktestSearch({ symbols: [" btcusdt ", "BTCUSDT"] }),
		).toEqual({ symbol: "BTCUSDT" });
	});

	it("keeps the strategy, normalized coin and favorites filter, dropping old state", () => {
		expect(
			parseStrategyBacktestSearch({
				all_coins: true,
				hold: 24,
				period: 30,
				sort_column: "symbol",
				sort_direction: "asc",
				strategy: "0199a3f2-5c1e-7b2a-9d4e-6f8a1b2c3d4e",
				symbol: " btcusdt ",
				symbol_filter: "BTC",
			}),
		).toEqual({
			all_coins: true,
			strategy: "0199a3f2-5c1e-7b2a-9d4e-6f8a1b2c3d4e",
			symbol: "BTCUSDT",
		});
	});

	it.each([4, "4", "0199a3f2-5c1e-7b2a-9d4e", null])(
		"drops the invalid strategy id %s",
		(strategy) => {
			expect(parseStrategyBacktestSearch({ strategy })).toEqual({});
		},
	);

	it.each(["", "   ", 123, null, ["BTCUSDT"]])(
		"drops the invalid coin %s",
		(symbol) => {
			expect(parseStrategyBacktestSearch({ symbol })).toEqual({});
		},
	);

	it.each(["true", false, 1])("drops the invalid all_coins %s", (allCoins) => {
		expect(parseStrategyBacktestSearch({ all_coins: allCoins })).toEqual({});
	});

	it("keeps the period days and drops other values", () => {
		expect(
			parseStrategyBacktestSearch({ from: "2026-01-01", to: "2026-03-31" }),
		).toEqual({ from: "2026-01-01", to: "2026-03-31" });
		expect(
			parseStrategyBacktestSearch({ from: "2026-02-30", to: 20260331 }),
		).toEqual({});
	});
});
