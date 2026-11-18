import { describe, expect, it } from "vitest";

import type { GridLimitsResponse } from "@/api/generated/models";
import { GRID_MAX_COUNT } from "@/features/instrument-analysis/grid-estimator/calculator/grid";
import {
	gridCountCap,
	gridCountForRange,
	gridCountForStep,
	gridCountStepError,
	gridCountStepPercent,
} from "@/features/instrument-analysis/grid-estimator/grid-range";

describe("grid count step", () => {
	const range = { gridCount: "4", lowerPrice: "90", upperPrice: "110" };

	it("does not depend on the investment", () => {
		expect(gridCountStepPercent(range, "geometric")).toBeGreaterThan(5);
	});

	it("rejects a step outside the hourly-to-daily range", () => {
		expect(gridCountStepError(2, 1, 3)).toBeNull();
		expect(gridCountStepError(0.5, 1, 3)).toContain("from 1% to 3%");
		expect(gridCountStepError(4, 1, null)).toBeNull();
		expect(gridCountStepError(0.5, 1, null)).toContain("at least 1%");
		expect(gridCountStepError(0.5, null, 3)).toBeNull();
	});
});

describe("gridCountForRange", () => {
	const range = {
		gridType: "geometric",
		lowerPrice: "1230",
		rangePercent: 1,
		upperPrice: "1640",
	} as const;

	it("derives the count of a range wide enough for one step", () => {
		expect(gridCountForRange(range, "40", GRID_MAX_COUNT, null, 0)).toEqual({
			error: null,
			gridCount: gridCountForStep(
				"1640",
				"1230",
				1,
				"geometric",
				GRID_MAX_COUNT,
			),
		});
	});

	it("caps the derived count", () => {
		expect(gridCountForRange(range, "40", 20, null, 0)).toEqual({
			error: null,
			gridCount: "20",
		});
	});

	it("keeps the count and explains a range narrower than one step", () => {
		expect(
			gridCountForRange(
				{ ...range, lowerPrice: "1639" },
				"40",
				GRID_MAX_COUNT,
				null,
				0,
			),
		).toEqual({
			error:
				"The price range is narrower than one 1% step, so the grid count is unchanged",
			gridCount: "40",
		});
	});

	it("keeps the count without an error for an invalid range or no step", () => {
		expect(
			gridCountForRange(
				{ ...range, upperPrice: "" },
				"40",
				GRID_MAX_COUNT,
				null,
				0,
			),
		).toEqual({ error: null, gridCount: "40" });
		expect(
			gridCountForRange(
				{ ...range, rangePercent: 0 },
				"40",
				GRID_MAX_COUNT,
				null,
				0,
			),
		).toEqual({ error: null, gridCount: "40" });
	});
});

describe("gridCountCap", () => {
	const limits: GridLimitsResponse = {
		coinm: null,
		spot: {
			ask_multiplier_up: 5,
			average_price: 62530,
			bid_multiplier_down: 0.2,
			max_price: 0,
			min_notional: 5,
			min_price: 0.01,
			min_qty: 0.00001,
			step_size: 0.00001,
			tick_size: 0.01,
		},
		symbol: "BTCUSDT",
		usdm: null,
	};
	const range = {
		gridType: "arithmetic",
		lowerPrice: "55000",
		upperPrice: "70000",
	} as const;

	it("is the Binance maximum of the market", () => {
		expect(gridCountCap("spot", range, limits)).toBe(107);
	});

	it("is the market's cap for a range too narrow for a grid", () => {
		expect(
			gridCountCap(
				"spot",
				{ ...range, lowerPrice: "62500", upperPrice: "62600" },
				limits,
			),
		).toBe(500);
	});

	it("falls back to the calculator maximum without the market's limits", () => {
		expect(gridCountCap("usdm", range, limits)).toBe(GRID_MAX_COUNT);
	});
});
