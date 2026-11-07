import Decimal from "decimal.js";
import { describe, expect, it } from "vitest";

import {
	gridBounds,
	latestAvailableCandle,
	lowerMarkupPercent,
	lowerPriceFromMarkup,
	upperMarkupPercent,
	upperPriceFromMarkup,
} from "@/features/instrument-analysis/grid-estimator/bounds";
import { DEFAULT_INVESTMENT } from "@/features/instrument-analysis/grid-estimator/config";
import {
	defaultGridStepPercent,
	defaultInvestment,
	gridRecommendation,
	investmentFromUsdt,
	savedGridFormValues,
} from "@/features/instrument-analysis/grid-estimator/defaults";
import {
	calculateSpotGridInput,
	spotGridMinimumStepPercent,
} from "@/features/instrument-analysis/grid-estimator/estimate";
import {
	evenLowerBound,
	evenUpperBound,
	gridCountForStep,
} from "@/features/instrument-analysis/grid-estimator/grid-range";
import type { GridBounds } from "@/features/instrument-analysis/grid-estimator/types";
import type { GridType } from "@/utils/calculator/types";

function recommend(
	bounds: GridBounds,
	stepPercent: number | undefined,
	gridType: GridType = "geometric",
) {
	return gridRecommendation(bounds, stepPercent, gridType, DEFAULT_INVESTMENT);
}

describe("defaultGridStepPercent", () => {
	it.each([
		["stays at the hourly range up to the low ratio", 1, 3, 1],
		["moves towards the daily range with the ratio", 1, 5.5, 2.6875],
		["stops at its maximum position from the high ratio", 1, 8, 6.25],
		["keeps the maximum position above the high ratio", 1, 21, 16],
		["stays at the hourly range without a larger daily range", 2, 1, 2],
	])("%s", (_, hourly, daily, expected) => {
		expect(defaultGridStepPercent(hourly, daily)).toBeCloseTo(expected);
	});

	it("falls back to the hourly range without a daily range", () => {
		expect(defaultGridStepPercent(1, undefined)).toBe(1);
		expect(defaultGridStepPercent(undefined, 5)).toBeUndefined();
	});
});

describe("savedGridFormValues", () => {
	it("keeps the values and takes the grid count of the new step", () => {
		const bounds = gridBounds("usdm", undefined, undefined);
		const values = {
			direction: "long",
			gridCount: "99",
			gridType: "geometric",
			investment: "2000",
			leverage: 3,
			lowerMarkup: 10,
			lowerPrice: "90",
			markup: 10,
			rangePercent: 1,
			upperPrice: "110",
		} as const;

		expect(savedGridFormValues({ bounds, values }, 5)).toEqual({
			...values,
			gridCount: "4",
			rangePercent: 5,
		});
	});

	it("restores a typed price outside the limits from its markup", () => {
		const bounds = {
			...gridBounds("usdm", undefined, undefined),
			anchor: 100,
			minPrice: new Decimal(32),
		};
		const { lowerPrice } = savedGridFormValues(
			{
				bounds,
				values: {
					direction: "long",
					gridCount: "99",
					gridType: "geometric",
					investment: "2000",
					leverage: 1,
					lowerMarkup: 10,
					lowerPrice: "1",
					markup: 10,
					rangePercent: 1,
					upperPrice: "110",
				},
			},
			1,
		);
		expect(lowerPrice).toBe("90");
	});
});

describe("default range", () => {
	const bounds = { ...gridBounds("usdm", undefined, undefined), anchor: 100 };

	it.each(["geometric", "arithmetic"] as const)(
		"widens a %s range to fit two steps",
		(gridType) => {
			const { input, lowerMarkup, upperMarkup } = recommend(
				bounds,
				25,
				gridType,
			);
			expect(input.gridCount).toBe("2");
			expect(lowerMarkup).toBe(upperMarkup);
			expect(lowerMarkup).toBeGreaterThan(10);
		},
	);

	it("stays at the minimum markup when two steps fit", () => {
		expect(recommend(bounds, 9)).toMatchObject({
			lowerMarkup: 10,
			upperMarkup: 10,
		});
	});

	it("stops at the limits, where the step may not fit", () => {
		expect(
			recommend({ ...bounds, lowerMarkupMax: 12, upperMarkupMax: 15 }, 90),
		).toMatchObject({ lowerMarkup: 12, upperMarkup: 15 });
	});
});

describe("spot grid recommendations", () => {
	const candle = {
		close: 100,
		high: 101,
		low: 98,
		open: 99,
		open_time: "2026-09-01T00:00:00Z",
	};
	const bounds = gridBounds("spot", [candle, null], null);
	const at = (anchor: number) => ({ ...bounds, anchor });

	it("uses Decimal arithmetic for 0%, 5%, and 50% markup", () => {
		expect(upperPriceFromMarkup(bounds, 0)).toBe("100");
		expect(upperPriceFromMarkup(bounds, 5)).toBe("105");
		expect(upperPriceFromMarkup(bounds, 50)).toBe("150");
	});

	function minimumStep(
		input: Parameters<typeof calculateSpotGridInput>[0],
		gridType: GridType,
	): number {
		const calculation = calculateSpotGridInput(input, gridType);
		expect(calculation?.error).toBeNull();
		if (!calculation?.estimate) throw new Error("missing estimate");
		return spotGridMinimumStepPercent(calculation.estimate);
	}

	it("starts both prices at the default markup within the range", () => {
		const recommendation = recommend(bounds, 1);
		expect(latestAvailableCandle([candle, null])).toEqual(candle);
		expect(recommendation).toMatchObject({ lowerMarkup: 10, upperMarkup: 10 });
		expect(recommendation.input).toEqual({
			lowerPrice: "90",
			upperPrice: "110",
			gridCount: "20",
			investment: "1000",
		});
		expect(
			minimumStep(recommendation.input, "geometric"),
		).toBeGreaterThanOrEqual(1);
		expect(
			recommend({ ...bounds, lowerMarkupMax: 4, upperMarkupMax: 6 }, 1),
		).toMatchObject({ lowerMarkup: 4, upperMarkup: 6 });
	});

	it("derives the arithmetic grid count from the same range", () => {
		const recommendation = recommend(bounds, 1, "arithmetic");
		expect(recommendation.input).toMatchObject({
			lowerPrice: "90",
			gridCount: "18",
		});
		expect(
			minimumStep(recommendation.input, "arithmetic"),
		).toBeGreaterThanOrEqual(1);
	});

	it("uses the shared number formatter for calculator-safe rounded prices", () => {
		expect(upperPriceFromMarkup(at(12_345_678.9), 0)).toBe("12345679");
		expect(lowerPriceFromMarkup(at(105), 33)).toBe("70.35");
	});

	it("converts between prices and markups", () => {
		expect(lowerPriceFromMarkup(at(1640), 25)).toBe("1230");
		expect(lowerPriceFromMarkup(at(105), 0)).toBe("105");
		expect(lowerMarkupPercent(1640, "1230")).toBe(25);
		expect(lowerMarkupPercent(105, "70.3")).toBe(33.05);
		expect(upperMarkupPercent(100, "105")).toBe(5);
		expect(upperMarkupPercent(100, "95")).toBe(-5);
	});

	it.each(["geometric", "arithmetic"] as const)(
		"derives the largest %s grid count that keeps the minimum step",
		(gridType) => {
			for (const stepPercent of [0.3, 0.705, 1, 2.5]) {
				const gridCount = gridCountForStep(
					"1640",
					"1230",
					stepPercent,
					gridType,
				);
				if (!gridCount) throw new Error("missing grid count");
				const input = {
					lowerPrice: "1230",
					upperPrice: "1640",
					gridCount,
					investment: "1000",
				};
				expect(minimumStep(input, gridType)).toBeGreaterThanOrEqual(
					stepPercent,
				);
				expect(
					minimumStep(
						{ ...input, gridCount: String(Number(gridCount) + 1) },
						gridType,
					),
				).toBeLessThan(stepPercent);
			}
		},
	);

	it("does not add a grid when rounding a markup-derived lower price down", () => {
		expect(lowerPriceFromMarkup(at(2.13), 25)).toBe("1.597");
		expect(lowerPriceFromMarkup(at(2130), 25)).toBe("1597");
		expect(gridCountForStep("2130", "1597", 0.722, "geometric")).toBe("40");
		expect(gridCountForStep("2130", "1597", 0.722, "geometric", 2130, 25)).toBe(
			"39",
		);
		const input = {
			lowerPrice: "1597",
			upperPrice: "2130",
			gridCount: "39",
			investment: "1000",
		};
		expect(minimumStep(input, "geometric")).toBeGreaterThanOrEqual(0.722);
	});

	it.each(["geometric", "arithmetic"] as const)(
		"moves a %s range bound inward until the grid is even",
		(gridType) => {
			const range = {
				gridType,
				lowerPrice: "90",
				rangePercent: 1,
				upperPrice: "110.5",
			};
			const ticked = { ...bounds, tickSize: new Decimal("0.0001") };
			const gridCount = gridCountForStep("110.5", "90", 1, gridType);
			const upper = evenUpperBound(ticked, range, 10);
			const lower = evenLowerBound(ticked, range, 10);
			if (!upper || !lower || !gridCount) throw new Error("missing bound");
			expect(Number(upper.upperPrice)).toBeLessThan(110.5);
			expect(Number(lower.lowerPrice)).toBeGreaterThan(90);

			for (const even of [
				{ ...range, upperPrice: upper.upperPrice },
				{ ...range, lowerPrice: lower.lowerPrice },
			]) {
				const markup = lowerMarkupPercent(100, even.lowerPrice, "ceil") ?? 0;
				expect(
					gridCountForStep(even.upperPrice, even.lowerPrice, 1, gridType),
				).toBe(gridCount);
				expect(
					minimumStep(
						{
							gridCount,
							investment: "1000",
							lowerPrice: even.lowerPrice,
							upperPrice: even.upperPrice,
						},
						gridType,
					),
				).toBeCloseTo(1, 4);
				expect(evenUpperBound(ticked, even, markup)).toBeNull();
				expect(evenLowerBound(ticked, even, markup)).toBeNull();
			}
		},
	);

	it("aligns the upper price above a lower price rounded down from its markup", () => {
		const ticked = {
			...bounds,
			anchor: 2.13,
			tickSize: new Decimal("0.0001"),
		};
		const lowerPrice = lowerPriceFromMarkup(ticked, 14.56);
		if (!lowerPrice) throw new Error("missing lower price");
		const range = {
			gridType: "geometric" as const,
			lowerPrice,
			rangePercent: 0.722,
			upperPrice: "2.4469",
		};
		const upper = evenUpperBound(ticked, range, 14.56);
		if (!upper) throw new Error("missing upper bound");
		const even = { ...range, upperPrice: upper.upperPrice };
		expect(
			gridCountForStep(
				even.upperPrice,
				lowerPrice,
				0.722,
				"geometric",
				2.13,
				14.56,
			),
		).toBe(
			gridCountForStep(
				range.upperPrice,
				lowerPrice,
				0.722,
				"geometric",
				2.13,
				14.56,
			),
		);
		expect(evenUpperBound(ticked, even, 14.56)).toBeNull();
	});

	it("keeps a range bound when an even grid would cross the anchor or fit no step", () => {
		const range = {
			gridType: "geometric" as const,
			lowerPrice: "90",
			rangePercent: 1,
			upperPrice: "100.2",
		};
		expect(evenUpperBound(bounds, range, 10)).toBeNull();
		expect(
			evenLowerBound(
				bounds,
				{ ...range, lowerPrice: "99.9", upperPrice: "100.5" },
				0.1,
			),
		).toBeNull();
		// More than the maximum grid count fits, so the steps stay wider.
		const capped = { ...range, rangePercent: 0.1, upperPrice: "100" };
		expect(
			evenUpperBound(at(10), { ...capped, lowerPrice: "9" }, 10),
		).toBeNull();
	});

	it("treats a range within price rounding of even as even", () => {
		expect(
			evenLowerBound(
				at(18.65),
				{
					gridType: "arithmetic",
					lowerPrice: "10.02",
					rangePercent: 2.768,
					upperPrice: "33.47",
				},
				46.23,
			),
		).toBeNull();
		const ticked = {
			...at(18.125),
			tickSize: new Decimal("0.0001"),
		};
		expect(
			evenUpperBound(
				ticked,
				{
					gridType: "geometric",
					lowerPrice: "17.9387",
					rangePercent: 1.442,
					upperPrice: "24.9346",
				},
				lowerMarkupPercent(18.125, "17.9387", "ceil") ?? 0,
			),
		).toBeNull();
	});

	it("keeps a typed grid count and its even range", () => {
		const input = {
			gridCount: "3",
			investment: "1000",
			lowerPrice: "90",
			upperPrice: "110",
		};
		const stepPercent = minimumStep(input, "geometric");
		expect(gridCountForStep("110", "90", stepPercent, "geometric")).toBe("3");
		const range = {
			gridType: "geometric" as const,
			lowerPrice: "90",
			rangePercent: stepPercent,
			upperPrice: "110",
		};
		expect(evenUpperBound(bounds, range, 10)).toBeNull();
		expect(evenLowerBound(bounds, range, 10)).toBeNull();
	});

	it("returns partial fallbacks for missing market data and unsupported values", () => {
		expect(latestAvailableCandle([null, null])).toBeNull();
		expect(recommend(bounds, 0)).toMatchObject({
			input: { lowerPrice: "90", upperPrice: "110", gridCount: "40" },
		});
		expect(
			recommend(gridBounds("spot", [{ ...candle, close: Number.NaN }], null), 1)
				.input,
		).toEqual({
			lowerPrice: "",
			upperPrice: "",
			gridCount: "40",
			investment: "1000",
		});
		expect(lowerPriceFromMarkup(at(Number.POSITIVE_INFINITY), 10)).toBeNull();
		expect(lowerPriceFromMarkup(at(105), -1)).toBeNull();
		expect(lowerPriceFromMarkup(at(105), 100)).toBeNull();
		expect(lowerMarkupPercent(105, "")).toBeNull();
		expect(upperMarkupPercent(null, "105")).toBeNull();
		expect(gridCountForStep("105", "105", 1, "geometric")).toBeNull();
		expect(gridCountForStep("105", "104.9", 1, "geometric")).toBeNull();
		expect(gridCountForStep("105", "70", 0, "geometric")).toBeNull();
		expect(gridCountForStep("105", "0.0001", 0.001, "geometric")).toBe("1000");
	});
});

describe("investments", () => {
	it("converts USDT into the coin only for COIN-M grids", () => {
		expect(investmentFromUsdt("usdm", null, "1000")).toBe("1000");
		expect(investmentFromUsdt("coinm", 40_000, "1000")).toBe("0.025");
		expect(investmentFromUsdt("coinm", 0.1, "1000")).toBe("10000");
		expect(investmentFromUsdt("coinm", null, "1000")).toBeNull();
	});

	it("starts COIN-M grids at the default worth, or one coin without a price", () => {
		expect(defaultInvestment("spot", null)).toBe("1000");
		expect(defaultInvestment("coinm", 3)).toBe("333.3");
		expect(defaultInvestment("coinm", null)).toBe("1");
	});
});
