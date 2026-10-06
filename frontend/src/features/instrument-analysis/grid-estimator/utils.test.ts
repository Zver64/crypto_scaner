import Decimal from "decimal.js";
import { describe, expect, it } from "vitest";
import {
	DEFAULT_MARKUPS,
	LOWER_MARKUP_MAX_PERCENT,
	UPPER_MARKUP_MAX_PERCENT,
} from "@/features/instrument-analysis/grid-estimator/config";
import {
	calculateFuturesGridInput,
	calculateSpotGridInput,
	defaultInvestment,
	futuresGridEstimateValues,
	gridBounds,
	gridCountForRange,
	gridCountForStep,
	gridMarketEstimate,
	gridRecommendation,
	investmentFromUsdt,
	latestAvailableCandle,
	liquidationRangeBar,
	lowerMarkupPercent,
	lowerPriceFromMarkup,
	lowerPriceLimitError,
	markupScaleLabels,
	profitSplitRows,
	spotGridEstimateValues,
	spotGridMinimumStepPercent,
	spotGridProfitSplits,
	upperMarkupPercent,
	upperPriceFromMarkup,
	upperPriceLimitError,
} from "@/features/instrument-analysis/grid-estimator/utils";
import type { GridType } from "@/utils/calculator/types";

const validInput = {
	lowerPrice: "100",
	upperPrice: "120",
	gridCount: "2",
	investment: "220",
};

const zeroValues = {
	profitSplits: [
		{
			cleanProfit: "0 USDT",
			cleanSegmentPercent: 0,
			cleanReturnPercent: "0% per trade",
			feeCost: "0 USDT",
			feeSegmentPercent: 0,
			feeShareOfGross: "0% of gross",
			isLoss: false,
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

	it("starts the lower price as low as the range allows", () => {
		const recommendation = gridRecommendation(bounds, 1);
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

	it("starts both USDT-M prices at the default markups within the range", () => {
		const recommendation = gridRecommendation(
			bounds,
			1,
			"geometric",
			DEFAULT_MARKUPS.usdm,
		);
		expect(recommendation.input).toMatchObject({
			lowerPrice: "90",
			upperPrice: "110",
			gridCount: "20",
		});
		expect(
			gridRecommendation(
				{ ...bounds, lowerMarkupMax: 4, upperMarkupMax: 6 },
				1,
				"geometric",
				DEFAULT_MARKUPS.usdm,
			),
		).toMatchObject({ lowerMarkup: 4, upperMarkup: 6 });
	});

	it("derives the arithmetic grid count from the same range", () => {
		const recommendation = gridRecommendation(bounds, 1, "arithmetic");
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

	it("returns partial fallbacks for missing market data and unsupported values", () => {
		expect(latestAvailableCandle([null, null])).toBeNull();
		expect(gridRecommendation(bounds, 0)).toMatchObject({
			input: { lowerPrice: "50", upperPrice: "105", gridCount: "40" },
		});
		expect(
			gridRecommendation(
				gridBounds("spot", [{ ...candle, close: Number.NaN }], null),
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
			profitSplits: [
				{
					cleanProfit: "10.76 USDT",
					cleanSegmentPercent: 97.8011,
					cleanReturnPercent: "9.78% per trade",
					feeCost: "0.2419 USDT",
					feeSegmentPercent: 2.1989,
					feeShareOfGross: "2.199% of gross",
					isLoss: false,
				},
			],
		});
	});

	it("pairs arithmetic lowest- and highest-profit trade splits", () => {
		const calculation = calculateSpotGridInput(validInput, "arithmetic");
		expect(calculation?.estimate).not.toBeNull();
		if (!calculation?.estimate) return;

		expect(spotGridProfitSplits(calculation.estimate)).toEqual([
			{
				cleanProfit: "9.76 USDT",
				cleanSegmentPercent: 97.6012,
				cleanReturnPercent: "8.873% per trade",
				feeCost: "0.2399 USDT",
				feeSegmentPercent: 2.3988,
				feeShareOfGross: "2.399% of gross",
				isLoss: false,
				label: "Lowest-profit trade",
			},
			{
				cleanProfit: "10.76 USDT",
				cleanSegmentPercent: 97.8011,
				cleanReturnPercent: "9.78% per trade",
				feeCost: "0.2419 USDT",
				feeSegmentPercent: 2.1989,
				feeShareOfGross: "2.199% of gross",
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
		expect(doubledSplits.map((split) => split.cleanProfit)).toEqual([
			"19.52 USDT",
			"21.52 USDT",
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
				cleanProfit: "-0.1001 USDT",
				cleanSegmentPercent: 0,
				cleanReturnPercent: "-0.1001% per trade",
				feeCost: "0.2001 USDT",
				feeSegmentPercent: 100,
				feeShareOfGross: "200.1% of gross",
				isLoss: true,
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

describe("calculateFuturesGridInput", () => {
	const options = {
		currentPrice: 105,
		direction: "short",
		gridType: "geometric",
		leverage: 3,
	} as const;
	const futuresInput = { ...validInput, upperPrice: "121", gridCount: "2" };

	it("shows one geometric trade", () => {
		const calculation = calculateFuturesGridInput(futuresInput, options);
		const values = futuresGridEstimateValues(
			calculation?.estimate ?? null,
			"USDT",
		);

		expect(calculation?.error).toBeNull();
		expect(values.profitSplits.map((split) => split.label)).toEqual([
			undefined,
		]);
	});

	it("pairs the lowest- and highest-profit arithmetic trades", () => {
		const calculation = calculateFuturesGridInput(futuresInput, {
			...options,
			gridType: "arithmetic",
		});

		expect(
			futuresGridEstimateValues(
				calculation?.estimate ?? null,
				"USDT",
			).profitSplits.map((split) => split.label),
		).toEqual(["Lowest-profit trade", "Highest-profit trade"]);
	});

	it("shows COIN-M geometric trades in the coin, which differ by level", () => {
		const estimate = gridMarketEstimate(
			"coinm",
			{ ...futuresInput, investment: "2" },
			options,
			"BTC",
		);

		expect(estimate.error).toBeNull();
		expect(
			estimate.values.profitSplits.map(({ cleanProfit, label }) => [
				label,
				cleanProfit,
			]),
		).toEqual([
			["Lowest-profit trade", "0.2592 BTC"],
			["Highest-profit trade", "0.2852 BTC"],
		]);
	});

	it("reports a missing current price as an input error", () => {
		const calculation = calculateFuturesGridInput(futuresInput, {
			...options,
			currentPrice: null,
		});

		expect(calculation?.estimate).toBeNull();
		expect(calculation?.error).toBe("The current price is unavailable");
	});
});

describe("liquidationRangeBar", () => {
	const lower = new Decimal(100);
	const upper = new Decimal(200);

	it("places a liquidation below the grid on a padded common scale", () => {
		const bar = liquidationRangeBar(lower, upper, 150, new Decimal(80));

		// The scale spans 80–200 with 6 on each side.
		expect(bar.liquidationPosition).toBeCloseTo((6 / 132) * 100);
		expect(bar.gridStartPosition).toBeCloseTo((26 / 132) * 100);
		expect(bar.gridEndPosition).toBeCloseTo((126 / 132) * 100);
		expect(bar.currentPosition).toBeCloseTo((76 / 132) * 100);
		expect(bar.isInsideGrid).toBe(false);
		expect(bar.summary).toBe("Liquidation 20% below the lower price");
	});

	it("measures a short liquidation above the upper price", () => {
		const bar = liquidationRangeBar(lower, upper, 150, new Decimal(250));

		expect(bar.summary).toBe("Liquidation 25% above the upper price");
		expect(bar.liquidationPosition).toBeGreaterThan(bar.gridEndPosition);
	});

	it("flags a liquidation inside the grid range", () => {
		const bar = liquidationRangeBar(lower, upper, 190, new Decimal(120));

		expect(bar.isInsideGrid).toBe(true);
		expect(bar.summary).toBe("Liquidation inside the grid range");
	});

	it("shows only the grid without a liquidation price", () => {
		const bar = liquidationRangeBar(lower, upper, null, null);

		expect(bar.liquidationPosition).toBeNull();
		expect(bar.currentPosition).toBeNull();
		expect(bar.gridStartPosition).toBeCloseTo((5 / 110) * 100);
		expect(bar.summary).toBe("No liquidation");
	});
});

describe("gridBounds with Binance limits", () => {
	const limits = {
		askMultiplierUp: 1.5,
		averagePrice: 100,
		bidMultiplierDown: 0.25,
		maxPrice: 140,
		minPrice: 0.01,
		tickSize: 0.5,
	};
	const bounds = gridBounds("spot", undefined, limits);

	it("limits spot prices to the filter share and the symbol maximum", () => {
		// 100 × (1 − 0.75 × 0.85) = 36.25; min(140, 100 × (1 + 0.5 × 0.85)).
		expect(bounds.minPrice?.toString()).toBe("36.25");
		expect(bounds.maxPrice?.toString()).toBe("140");
		expect(bounds).toMatchObject({
			anchor: 100,
			lowerMarkupMax: 63.75,
			upperMarkupMax: 40,
		});
	});

	it("rounds markup maximums down and caps them", () => {
		expect(
			gridBounds("spot", undefined, { ...limits, bidMultiplierDown: 0.333 })
				.lowerMarkupMax,
		).toBe(56.69);
		expect(
			gridBounds("spot", undefined, {
				...limits,
				askMultiplierUp: 5,
				maxPrice: 0,
			}).upperMarkupMax,
		).toBe(200);
		expect(
			gridBounds("spot", undefined, {
				...limits,
				bidMultiplierDown: 0,
				minPrice: 0,
			}).lowerMarkupMax,
		).toBe(50);
	});

	it("rounds prices to the tick size and keeps them inside the limits", () => {
		expect(upperPriceFromMarkup(bounds, 5.3)).toBe("105.5");
		expect(lowerPriceFromMarkup(bounds, 10.3)).toBe("89.5");
		expect(upperPriceFromMarkup(bounds, 50)).toBe("140");
		expect(lowerPriceFromMarkup(bounds, 63.75)).toBe("36.5");
	});

	it("explains prices outside the limits", () => {
		expect(lowerPriceLimitError(bounds, "36", "USDT")).toBe(
			"Binance minimum is 36.5 USDT",
		);
		expect(lowerPriceLimitError(bounds, "36.25", "USDT")).toBeNull();
		expect(upperPriceLimitError(bounds, "140.5", "USDT")).toBe(
			"Binance maximum is 140 USDT",
		);
		expect(upperPriceLimitError(bounds, "140", "USDT")).toBeNull();
		expect(upperPriceLimitError(bounds, "", "USDT")).toBeNull();
	});

	it.each([
		"usdm",
		"coinm",
	] as const)("does not apply the spot limits to %s grids", (market) => {
		const futures = gridBounds(market, undefined, limits);

		expect(futures).toEqual({
			anchor: 100,
			lowerMarkupMax: LOWER_MARKUP_MAX_PERCENT,
			maxPrice: null,
			minPrice: null,
			tickSize: null,
			upperMarkupMax: UPPER_MARKUP_MAX_PERCENT,
		});
		expect(upperPriceLimitError(futures, "1000", "USDT")).toBeNull();
		expect(lowerPriceFromMarkup(futures, 10.3)).toBe("89.7");
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

describe("markupScaleLabels", () => {
	it("labels the default markup only inside the slider", () => {
		expect(markupScaleLabels(50, 5)).toEqual([
			{ label: "0%", position: 0 },
			{ label: "5%", position: 10 },
			{ label: "50%", position: 100 },
		]);
		expect(markupScaleLabels(4, 10)).toEqual([
			{ label: "0%", position: 0 },
			{ label: "4%", position: 100 },
		]);
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
		expect(gridCountForRange(range, "40", null, 0)).toEqual({
			error: null,
			gridCount: gridCountForStep("1640", "1230", 1, "geometric"),
		});
	});

	it("keeps the count and explains a range narrower than one step", () => {
		expect(
			gridCountForRange({ ...range, lowerPrice: "1639" }, "40", null, 0),
		).toEqual({
			error:
				"The price range is narrower than one 1% step, so the grid count is unchanged",
			gridCount: "40",
		});
	});

	it("keeps the count without an error for an invalid range or no step", () => {
		expect(
			gridCountForRange({ ...range, upperPrice: "" }, "40", null, 0),
		).toEqual({ error: null, gridCount: "40" });
		expect(
			gridCountForRange({ ...range, rangePercent: 0 }, "40", null, 0),
		).toEqual({ error: null, gridCount: "40" });
	});
});

describe("profitSplitRows", () => {
	it("leaves a single trade unlabelled", () => {
		const colors = { fee: "orange", profit: "green" };
		const rows = profitSplitRows(
			spotGridEstimateValues(null).profitSplits,
			colors,
		);

		expect(rows.map(({ key, label }) => ({ key, label }))).toEqual([
			{ key: "every-trade", label: undefined },
		]);
	});
});
