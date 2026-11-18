import { describe, expect, it } from "vitest";

import type { GridLimitsResponse } from "@/api/generated/models";
import { GridDecimal } from "@/features/instrument-analysis/grid-estimator/calculator/grid";
import {
	gridMaxCount,
	marketGridCountLimitParams,
} from "@/features/instrument-analysis/grid-estimator/calculator/grid-count-limit";
import type { GridType } from "@/features/instrument-analysis/grid-estimator/calculator/types";
import type { GridMarket } from "@/features/instrument-analysis/grid-estimator/types";

// Limits of a symbol listed on every market with the same tick size.
function limitsWithTick(tickSize: number): GridLimitsResponse {
	return {
		coinm: {
			contract_size: 100,
			equal_qty_precision: 6,
			mark_price: 1,
			min_qty: 1,
			step_size: 1,
			symbol: "TESTUSD_PERP",
			tick_size: tickSize,
		},
		spot: {
			ask_multiplier_up: 5,
			average_price: 1,
			bid_multiplier_down: 0.2,
			max_price: 0,
			min_notional: 5,
			min_price: 0,
			min_qty: 1,
			step_size: 1,
			tick_size: tickSize,
		},
		symbol: "TESTUSDT",
		usdm: {
			mark_price: 1,
			min_notional: 5,
			min_qty: 1,
			step_size: 1,
			symbol: "TESTUSDT",
			tick_size: tickSize,
		},
	};
}

// The largest grid count of a market at its default fee, or at `fee`.
function marketMax(
	market: GridMarket,
	upper: string,
	lower: string,
	tickSize: number,
	gridType: GridType,
	fee?: string,
): number {
	const params = marketGridCountLimitParams(market, limitsWithTick(tickSize));
	if (!params) throw new Error("missing parameters");
	return gridMaxCount(
		new GridDecimal(upper),
		new GridDecimal(lower),
		gridType,
		fee === undefined ? params : { ...params, fee: new GridDecimal(fee) },
	);
}

describe("gridMaxCount", () => {
	it.each([
		["70000", "55000", 0.01, "arithmetic", undefined, 107],
		["70000", "55000", 0.01, "geometric", undefined, 120],
		["62500", "62000", 0.01, "arithmetic", undefined, 4],
		["62600", "62500", 0.01, "arithmetic", undefined, 0],
		["1.2345", "1.2", 0.0001, "arithmetic", undefined, 13],
		["1.2345", "1.2", 0.0001, "geometric", undefined, 14],
		["1.2021", "1.1", 0.0001, "arithmetic", undefined, 40],
		["1.2501", "1.1", 0.0001, "arithmetic", undefined, 57],
		["1000", "900", 0.5, "arithmetic", "0.0013", 38],
		["1000", "900", 0.1, "arithmetic", "0.00013", 333],
		["70000", "55000", 0.01, "arithmetic", "0", 500],
		["1.001", "1", 0.0001, "geometric", "0", 9],
		["55000", "70000", 0.01, "arithmetic", undefined, 0],
	] as const)(
		"limits a spot grid of %s–%s at tick %s (%s, fee %s) to %s",
		(upper, lower, tickSize, gridType, fee, expected) => {
			expect(marketMax("spot", upper, lower, tickSize, gridType, fee)).toBe(
				expected,
			);
		},
	);

	it.each([
		["70000", "55000", 0.1, "arithmetic", 535],
		["70000", "55000", 0.1, "geometric", 602],
		["0.01", "0.009", 0.000001, "arithmetic", 100],
		["0.01", "0.009", 0.000001, "geometric", 94],
		["100", "90", 0.00001, "arithmetic", 250],
		["100", "99.99", 0.000011, "arithmetic", 0],
	] as const)(
		"limits a USDT-M grid of %s–%s at tick %s (%s) to %s",
		(upper, lower, tickSize, gridType, expected) => {
			expect(marketMax("usdm", upper, lower, tickSize, gridType)).toBe(
				expected,
			);
		},
	);

	it("limits a COIN-M grid to its cap", () => {
		expect(marketMax("coinm", "70000", "55000", 0.1, "arithmetic")).toBe(169);
	});
});

describe("marketGridCountLimitParams", () => {
	it("needs the market's Binance limits", () => {
		expect(marketGridCountLimitParams("spot", null)).toBeNull();
		expect(
			marketGridCountLimitParams("usdm", {
				...limitsWithTick(0.1),
				usdm: null,
			}),
		).toBeNull();
		expect(
			marketGridCountLimitParams("coinm", {
				...limitsWithTick(0.1),
				coinm: null,
			}),
		).toBeNull();
	});
});
