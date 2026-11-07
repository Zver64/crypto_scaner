import Decimal from "decimal.js";
import { describe, expect, it } from "vitest";

import {
	calculateFuturesGridInput,
	calculateSpotGridInput,
	futuresGridEstimateValues,
	gridMarketEstimate,
	liquidationRangeBar,
	profitSplitRows,
	spotGridEstimateValues,
	spotGridProfitSplits,
} from "@/features/instrument-analysis/grid-estimator/estimate";

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

describe("calculateSpotGridInput", () => {
	it.each(Object.keys(validInput) as (keyof typeof validInput)[])(
		"clears the estimate for partial input missing %s",
		(field) => {
			const calculation = calculateSpotGridInput({
				...validInput,
				[field]: "",
			});

			expect(calculation).toBeNull();
			expect(spotGridEstimateValues(calculation?.estimate ?? null)).toEqual(
				zeroValues,
			);
		},
	);

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
