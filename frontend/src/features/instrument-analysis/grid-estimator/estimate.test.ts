import Decimal from "decimal.js";
import { describe, expect, it } from "vitest";

import type { GridLimitsResponse } from "@/api/generated/models";
import {
	calculateFuturesGridInput,
	calculateSpotGridInput,
	futuresGridEstimateValues,
	gridMarketEstimate,
	gridMinimumInvestment,
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
			const calculation = calculateSpotGridInput(
				{
					...validInput,
					[field]: "",
				},
				"arithmetic",
			);

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
		const validCalculation = calculateSpotGridInput(validInput, "arithmetic");

		expect(validCalculation?.estimate).not.toBeNull();

		const clearedCalculation = calculateSpotGridInput(
			{
				...validInput,
				investment: "",
			},
			"arithmetic",
		);
		expect(clearedCalculation).toBeNull();
		expect(
			spotGridEstimateValues(clearedCalculation?.estimate ?? null),
		).toEqual(zeroValues);

		const invalidCalculation = calculateSpotGridInput(
			{
				...validInput,
				upperPrice: "100",
			},
			"arithmetic",
		);
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
		const calculation = calculateFuturesGridInput(
			futuresInput,
			options,
			"linear",
		);
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
		const calculation = calculateFuturesGridInput(
			futuresInput,
			{
				...options,
				gridType: "arithmetic",
			},
			"linear",
		);

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
		const calculation = calculateFuturesGridInput(
			futuresInput,
			{
				...options,
				currentPrice: null,
			},
			"linear",
		);

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

describe("gridMinimumInvestment", () => {
	// Cases checked against Binance bots: BTC, 55000–70000, 100 arithmetic
	// grids, 10× leverage.
	const input = {
		gridCount: "100",
		investment: "1000",
		lowerPrice: "55000",
		upperPrice: "70000",
	};
	const spot = {
		ask_multiplier_up: 5,
		average_price: 62530,
		bid_multiplier_down: 0.2,
		max_price: 0,
		min_notional: 5,
		min_price: 0.01,
		min_qty: 0.00001,
		step_size: 0.00001,
		tick_size: 0.01,
	};
	const usdm = {
		mark_price: 62432,
		min_notional: 50,
		min_qty: 0.001,
		step_size: 0.001,
		symbol: "BTCUSDT",
		tick_size: 0.1,
	};
	const coinm = {
		contract_size: 100,
		equal_qty_precision: 4,
		mark_price: 62322.4,
		min_qty: 1,
		step_size: 1,
		symbol: "BTCUSD_PERP",
		tick_size: 0.1,
	};
	const limits: GridLimitsResponse = { coinm, spot, symbol: "BTCUSDT", usdm };
	const minimum = (
		market: "spot" | "usdm" | "coinm",
		direction: "long" | "short",
		limitsOverride: Partial<GridLimitsResponse> = {},
	) =>
		gridMinimumInvestment(
			market,
			input,
			{ direction, gridType: "arithmetic", leverage: 10 },
			{ ...limits, ...limitsOverride },
		)?.toFixed();

	it("reserves the spot orders at the average price", () => {
		// (606.022 + 1.5 × 0.0001 × 62530) / 0.95 = 647.79105…
		expect(minimum("spot", "long")).toBe("647.8");
	});

	it.each([
		[62432, "long", "1032.5"],
		[62427.1982009, "long", "1032.8"],
		[80000, "long", "789.1"],
		[80000, "short", "3219.4"],
		[50000, "long", "2367.2"],
		[50000, "short", "789.1"],
	] as const)(
		"reserves USDT-M margin at mark price %s for a %s grid",
		(markPrice, direction, expected) => {
			expect(
				minimum("usdm", direction, {
					usdm: { ...usdm, mark_price: markPrice },
				}),
			).toBe(expected);
		},
	);

	it.each([
		[62322.4, "long", "0.0269"],
		[62333, "short", "0.0267"],
	] as const)(
		"reserves COIN-M margin at mark price %s for a %s grid",
		(markPrice, direction, expected) => {
			expect(
				minimum("coinm", direction, {
					coinm: { ...coinm, mark_price: markPrice },
				}),
			).toBe(expected);
		},
	);

	it("has no minimum without the market's limits or a valid range", () => {
		expect(minimum("usdm", "long", { usdm: null })).toBeUndefined();
		expect(
			gridMinimumInvestment(
				"spot",
				{ ...input, upperPrice: "50000" },
				{ direction: "long", gridType: "arithmetic", leverage: 10 },
				limits,
			),
		).toBeNull();
	});
});
