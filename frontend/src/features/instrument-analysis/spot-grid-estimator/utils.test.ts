import { describe, expect, it } from "vitest";
import {
	calculateSpotGridInput,
	latestAvailableCandle,
	recommendedLowerPrice,
	recommendedUpperPrice,
	spotGridEstimateValues,
	spotGridRecommendation,
} from "@/features/instrument-analysis/spot-grid-estimator/utils";

const validInput = {
	lowerPrice: "100",
	upperPrice: "120",
	gridCount: "2",
	investment: "220",
};

const zeroValues = {
	averageEntryPrice: "0 USDT",
	gridStepPercent: "0%",
	profitPerStep: "0 USDT",
	profitPerStepPercent: "0%",
};

describe("spot grid recommendations", () => {
	const candle = {
		close: 99,
		high: 100,
		low: 98,
		open: 99,
		open_time: "2026-09-01T00:00:00Z",
	};

	it("uses Decimal arithmetic for 0%, 5%, and 50% markup", () => {
		expect(recommendedUpperPrice(100, 0)).toBe("100");
		expect(recommendedUpperPrice(100, 5)).toBe("105");
		expect(recommendedUpperPrice(100, 50)).toBe("150");
	});

	it("uses the latest non-null candle and raw hourly step", () => {
		const recommendation = spotGridRecommendation([candle, null], 1);
		expect(latestAvailableCandle([candle, null])).toEqual(candle);
		expect(recommendation.input).toEqual({
			lowerPrice: "70.5",
			upperPrice: "105",
			gridCount: "40",
			investment: "1000",
		});
		const calculation = calculateSpotGridInput(
			recommendation.input,
			"geometric",
		);
		expect(calculation?.estimate).toMatchObject({
			gridCount: 40,
			levelCount: 41,
		});
		if (calculation?.estimate && "stepPercent" in calculation.estimate) {
			expect(calculation.estimate.stepPercent.gte(1)).toBe(true);
		}
	});

	it("uses the shared number formatter for calculator-safe rounded prices", () => {
		expect(recommendedUpperPrice(12_345_678.9, 0)).toBe("12300000");
		expect(recommendedLowerPrice("105", 1, "40")).toBe("70.5");
	});

	it("keeps the arithmetic minimum grid step at or above the target", () => {
		const lowerPrice = recommendedLowerPrice("105", 1, "40", "arithmetic");
		expect(lowerPrice).toBe("63.4");

		const calculation = calculateSpotGridInput(
			{
				lowerPrice: lowerPrice ?? "",
				upperPrice: "105",
				gridCount: "40",
				investment: "1000",
			},
			"arithmetic",
		);
		expect(calculation?.error).toBeNull();
		if (calculation?.estimate && "stepPercentMinimum" in calculation.estimate) {
			expect(calculation.estimate.stepPercentMinimum.gte(1)).toBe(true);
		}
	});

	it("rejects arithmetic targets that cannot produce a positive lower price", () => {
		expect(recommendedLowerPrice("105", 3, "40", "arithmetic")).toBeNull();
	});

	it("calculates a valid geometric range for a slider commit using grid count 10", () => {
		const lowerPrice = recommendedLowerPrice("105", 1, "10");
		expect(lowerPrice).toBe("95");

		const calculation = calculateSpotGridInput(
			{
				lowerPrice: lowerPrice ?? "",
				upperPrice: "105",
				gridCount: "10",
				investment: "1000",
			},
			"geometric",
		);
		expect(calculation?.estimate).toMatchObject({
			gridCount: 10,
			levelCount: 11,
		});
		if (calculation?.estimate && "stepPercent" in calculation.estimate) {
			expect(calculation.estimate.stepPercent.gte(1)).toBe(true);
		}
	});

	it("returns partial fallbacks for missing market data and unsupported lower values", () => {
		expect(latestAvailableCandle([null, null])).toBeNull();
		expect(spotGridRecommendation([candle], 0).input).toMatchObject({
			lowerPrice: "",
			upperPrice: "105",
		});
		expect(
			spotGridRecommendation([{ ...candle, high: Number.NaN }], 1).input,
		).toEqual({
			lowerPrice: "",
			upperPrice: "",
			gridCount: "40",
			investment: "1000",
		});
		expect(recommendedLowerPrice("1e1001", 1, "40")).toBeNull();
		expect(recommendedLowerPrice("105", -1, "40")).toBeNull();
		expect(recommendedLowerPrice("105", 1, "0")).toBeNull();
		expect(recommendedLowerPrice("105", 1e-59, "40")).toBeNull();
	});
});

describe("calculateSpotGridInput", () => {
	it.each(
		Object.keys(validInput) as (keyof typeof validInput)[],
	)("clears the estimate for partial input missing %s", (field) => {
		const calculation = calculateSpotGridInput({ ...validInput, [field]: "" });

		expect(calculation).toBeNull();
		expect(spotGridEstimateValues(calculation?.estimate ?? null)).toEqual(
			zeroValues,
		);
	});

	it("calculates a geometric grid with a single constant profit per step", () => {
		const calculation = calculateSpotGridInput(
			{ ...validInput, upperPrice: "121" },
			"geometric",
		);

		expect(calculation?.error).toBeNull();
		expect(spotGridEstimateValues(calculation?.estimate ?? null)).toEqual({
			averageEntryPrice: "105 USDT",
			gridStepPercent: "10%",
			profitPerStep: "10.8 USDT",
			profitPerStepPercent: "9.78%",
		});
	});

	it("shows the arithmetic grid step percent range", () => {
		const calculation = calculateSpotGridInput(validInput, "arithmetic");

		expect(calculation?.error).toBeNull();
		expect(
			spotGridEstimateValues(calculation?.estimate ?? null).gridStepPercent,
		).toBe("9.09%–10%");
	});

	it("clears a valid estimate after cleared or invalid input", () => {
		const validCalculation = calculateSpotGridInput(validInput);

		expect(validCalculation?.estimate).not.toBeNull();

		const clearedCalculation = calculateSpotGridInput({
			...validInput,
			investment: "",
		});
		expect(clearedCalculation).toBeNull();
		expect(
			spotGridEstimateValues(clearedCalculation?.estimate ?? null),
		).toEqual(zeroValues);

		const invalidCalculation = calculateSpotGridInput({
			...validInput,
			upperPrice: "100",
		});
		expect(invalidCalculation?.estimate).toBeNull();
		expect(invalidCalculation?.error).toContain("Upper price");
		expect(
			spotGridEstimateValues(invalidCalculation?.estimate ?? null),
		).toEqual(zeroValues);
	});
});
