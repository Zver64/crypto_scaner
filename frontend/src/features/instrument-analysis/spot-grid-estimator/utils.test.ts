import { describe, expect, it } from "vitest";
import {
	calculateSpotGridInput,
	gridCountForStep,
	LOWER_MARKUP_MAX_PERCENT,
	latestAvailableCandle,
	lowerMarkupPercent,
	lowerPriceFromMarkup,
	type SpotGridType,
	spotGridBounds,
	spotGridEstimateValues,
	spotGridMinimumStepPercent,
	spotGridProfitSplits,
	spotGridRecommendation,
	upperMarkupPercent,
	upperPriceFromMarkup,
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
	profitSplits: [
		{
			cleanProfit: "0 USDT",
			cleanSegmentPercent: 0,
			cleanReturnPercent: "0% per trade",
			feeCost: "0 USDT",
			feeSegmentPercent: 0,
			feeShareOfGross: "0% of gross",
			grossProfit: "0 USDT",
			isLoss: false,
			label: "Every trade",
		},
	],
};

describe("spot grid recommendations", () => {
	const candle = {
		close: 100,
		high: 101,
		low: 98,
		open: 99,
		open_time: "2026-09-01T00:00:00Z",
	};
	const bounds = spotGridBounds([candle, null], null);
	const at = (anchor: number) => ({ ...bounds, anchor });

	it("uses Decimal arithmetic for 0%, 5%, and 50% markup", () => {
		expect(upperPriceFromMarkup(bounds, 0)).toBe("100");
		expect(upperPriceFromMarkup(bounds, 5)).toBe("105");
		expect(upperPriceFromMarkup(bounds, 50)).toBe("150");
	});

	function minimumStep(
		input: Parameters<typeof calculateSpotGridInput>[0],
		gridType: SpotGridType,
	): number {
		const calculation = calculateSpotGridInput(input, gridType);
		expect(calculation?.error).toBeNull();
		if (!calculation?.estimate) throw new Error("missing estimate");
		return spotGridMinimumStepPercent(calculation.estimate);
	}

	it("starts the lower price as low as the range allows", () => {
		const recommendation = spotGridRecommendation(bounds, 1);
		expect(latestAvailableCandle([candle, null])).toEqual(candle);
		expect(recommendation.lowerMarkup).toBe(LOWER_MARKUP_MAX_PERCENT);
		expect(recommendation.input).toEqual({
			lowerPrice: "50",
			upperPrice: "105",
			gridCount: "74",
			investment: "1000",
		});
		expect(
			minimumStep(recommendation.input, "geometric"),
		).toBeGreaterThanOrEqual(1);
	});

	it("derives the arithmetic grid count from the same range", () => {
		const recommendation = spotGridRecommendation(bounds, 1, "arithmetic");
		expect(recommendation.input).toMatchObject({
			lowerPrice: "50",
			gridCount: "52",
		});
		expect(
			minimumStep(recommendation.input, "arithmetic"),
		).toBeGreaterThanOrEqual(1);
	});

	it("uses the shared number formatter for calculator-safe rounded prices", () => {
		expect(upperPriceFromMarkup(at(12_345_678.9), 0)).toBe("12345679");
		expect(lowerPriceFromMarkup(at(105), 33)).toBe("70.3");
	});

	it("converts between prices and markups", () => {
		expect(lowerPriceFromMarkup(at(1640), 25)).toBe("1230");
		expect(lowerPriceFromMarkup(at(105), 0)).toBe("105");
		expect(lowerMarkupPercent(1640, "1230")).toBe(25);
		expect(lowerMarkupPercent(105, "70.3")).toBe(33.05);
		expect(upperMarkupPercent(100, "105")).toBe(5);
		expect(upperMarkupPercent(100, "95")).toBe(-5);
	});

	it.each([
		"geometric",
		"arithmetic",
	] as const)("derives the largest %s grid count that keeps the minimum step", (gridType) => {
		for (const stepPercent of [0.3, 0.705, 1, 2.5]) {
			const gridCount = gridCountForStep("1640", "1230", stepPercent, gridType);
			if (!gridCount) throw new Error("missing grid count");
			const input = {
				lowerPrice: "1230",
				upperPrice: "1640",
				gridCount,
				investment: "1000",
			};
			expect(minimumStep(input, gridType)).toBeGreaterThanOrEqual(stepPercent);
			expect(
				minimumStep(
					{ ...input, gridCount: String(Number(gridCount) + 1) },
					gridType,
				),
			).toBeLessThan(stepPercent);
		}
	});

	it("does not add a grid when rounding a markup-derived lower price down", () => {
		expect(lowerPriceFromMarkup(at(2130), 25)).toBe("1597");
		expect(lowerPriceFromMarkup(at(2.13), 25)).toBe("1.59");
		expect(gridCountForStep("2.13", "1.59", 0.712, "geometric")).toBe("41");
		expect(gridCountForStep("2.13", "1.59", 0.712, "geometric", 2.13, 25)).toBe(
			"40",
		);
		expect(gridCountForStep("1640", "1230", 0.712, "geometric", 1640, 25)).toBe(
			"40",
		);
		const input = {
			lowerPrice: "1.59",
			upperPrice: "2.13",
			gridCount: "40",
			investment: "1000",
		};
		expect(minimumStep(input, "geometric")).toBeGreaterThanOrEqual(0.712);
	});

	it("returns partial fallbacks for missing market data and unsupported values", () => {
		expect(latestAvailableCandle([null, null])).toBeNull();
		expect(spotGridRecommendation(bounds, 0)).toMatchObject({
			input: { lowerPrice: "50", upperPrice: "105", gridCount: "40" },
		});
		expect(
			spotGridRecommendation(
				spotGridBounds([{ ...candle, close: Number.NaN }], null),
				1,
			).input,
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
			profitSplits: [
				{
					cleanProfit: "10.8 USDT",
					cleanSegmentPercent: 97.8011,
					cleanReturnPercent: "9.78% per trade",
					feeCost: "0.242 USDT",
					feeSegmentPercent: 2.1989,
					feeShareOfGross: "2.2% of gross",
					grossProfit: "11 USDT",
					isLoss: false,
					label: "Every trade",
				},
			],
		});
	});

	it("shows the arithmetic grid step percent range", () => {
		const calculation = calculateSpotGridInput(validInput, "arithmetic");

		expect(calculation?.error).toBeNull();
		expect(
			spotGridEstimateValues(calculation?.estimate ?? null).gridStepPercent,
		).toBe("9.09%–10%");
	});

	it("pairs arithmetic lowest- and highest-profit trade splits", () => {
		const calculation = calculateSpotGridInput(validInput, "arithmetic");
		expect(calculation?.estimate).not.toBeNull();
		if (!calculation?.estimate) return;

		expect(spotGridProfitSplits(calculation.estimate)).toEqual([
			{
				cleanProfit: "9.76 USDT",
				cleanSegmentPercent: 97.6012,
				cleanReturnPercent: "8.87% per trade",
				feeCost: "0.24 USDT",
				feeSegmentPercent: 2.3988,
				feeShareOfGross: "2.4% of gross",
				grossProfit: "10 USDT",
				isLoss: false,
				label: "Lowest-profit trade",
			},
			{
				cleanProfit: "10.8 USDT",
				cleanSegmentPercent: 97.8011,
				cleanReturnPercent: "9.78% per trade",
				feeCost: "0.242 USDT",
				feeSegmentPercent: 2.1989,
				feeShareOfGross: "2.2% of gross",
				grossProfit: "11 USDT",
				isLoss: false,
				label: "Highest-profit trade",
			},
		]);
	});

	it("scales split amounts with investment without changing fee shares", () => {
		const original = calculateSpotGridInput(validInput, "arithmetic");
		const doubled = calculateSpotGridInput(
			{ ...validInput, investment: "440" },
			"arithmetic",
		);
		if (!original?.estimate || !doubled?.estimate) return;

		const originalSplits = spotGridProfitSplits(original.estimate);
		const doubledSplits = spotGridProfitSplits(doubled.estimate);
		expect(doubledSplits.map((split) => split.grossProfit)).toEqual([
			"20 USDT",
			"22 USDT",
		]);
		expect(doubledSplits.map((split) => split.feeShareOfGross)).toEqual(
			originalSplits.map((split) => split.feeShareOfGross),
		);
		expect(doubledSplits.map((split) => split.feeSegmentPercent)).toEqual(
			originalSplits.map((split) => split.feeSegmentPercent),
		);
	});

	it("caps the fee bar and exposes a net loss when fees exceed gross", () => {
		const calculation = calculateSpotGridInput(
			{
				lowerPrice: "100",
				upperPrice: "100.1",
				gridCount: "1",
				investment: "100",
			},
			"geometric",
		);
		if (!calculation?.estimate) return;

		expect(spotGridProfitSplits(calculation.estimate)).toEqual([
			{
				cleanProfit: "-0.1 USDT",
				cleanSegmentPercent: 0,
				cleanReturnPercent: "-0.1% per trade",
				feeCost: "0.2 USDT",
				feeSegmentPercent: 100,
				feeShareOfGross: "200% of gross",
				grossProfit: "0.1 USDT",
				isLoss: true,
				label: "Every trade",
			},
		]);
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
