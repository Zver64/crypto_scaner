import type Decimal from "decimal.js";
import {
	assertSupportedGridValue,
	geometricStepRatio,
} from "@/utils/calculator/grid";
import { spotGrid, spotGridCycle } from "@/utils/calculator/spot-grid";
import type { GridInput } from "@/utils/calculator/types";

export interface GeometricSpotGridEstimate {
	allocationPerBuy: Decimal;
	cycleProfit: Decimal;
	cycleProfitPercent: Decimal;
	stepPercent: Decimal;
}

/**
 * Estimates a geometric Binance spot grid using equal percentage spacing and
 * equal quote allocations. The upper level is a sell level, not a buy level.
 * All returned values are unrounded Decimal instances.
 */
export function calculateGeometricSpotGrid(
	input: GridInput,
): GeometricSpotGridEstimate {
	const { allocationPerBuy, gridCount, lowerPrice, upperPrice } = spotGrid(
		input,
		"geometric",
	);
	// Every geometric step shares one ratio, so every cycle earns the same.
	const stepRatio = geometricStepRatio(lowerPrice, upperPrice, gridCount);
	const cycle = spotGridCycle(allocationPerBuy, stepRatio);
	const stepPercent = stepRatio.minus(1).times(100);
	assertSupportedGridValue(stepPercent, "Step percent", true);

	return {
		allocationPerBuy,
		cycleProfit: cycle.profit,
		cycleProfitPercent: cycle.profitPercent,
		stepPercent,
	};
}
