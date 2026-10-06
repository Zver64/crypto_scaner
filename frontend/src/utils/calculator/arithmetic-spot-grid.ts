import type Decimal from "decimal.js";
import { assertSupportedGridValue, extremes } from "@/utils/calculator/grid";
import { spotGrid, spotGridCycle } from "@/utils/calculator/spot-grid";
import type { GridInput } from "@/utils/calculator/types";

export interface ArithmeticSpotGridEstimate {
	allocationPerBuy: Decimal;
	cycleProfitMaximum: Decimal;
	cycleProfitMaximumPercent: Decimal;
	cycleProfitMinimum: Decimal;
	cycleProfitMinimumPercent: Decimal;
	stepPercentMaximum: Decimal;
	stepPercentMinimum: Decimal;
}

/**
 * Estimates an arithmetic Binance spot grid using equal quote allocations.
 * All returned values are unrounded Decimal instances.
 */
export function calculateArithmeticSpotGrid(
	input: GridInput,
): ArithmeticSpotGridEstimate {
	const { allocationPerBuy, gridCount, levels, lowerPrice, upperPrice } =
		spotGrid(input, "arithmetic");
	const cycles = levels
		.slice(0, -1)
		.map((buyPrice, index) =>
			spotGridCycle(allocationPerBuy, levels[index + 1].div(buyPrice)),
		);
	const { maximum, minimum } = extremes(cycles, (cycle) => cycle.profit);

	const stepPrice = upperPrice.minus(lowerPrice).div(gridCount);
	const highestBuyPrice = upperPrice.minus(stepPrice);
	const stepPercentMaximum = stepPrice.div(lowerPrice).times(100);
	const stepPercentMinimum = stepPrice.div(highestBuyPrice).times(100);
	assertSupportedGridValue(stepPercentMaximum, "Maximum step percent", true);
	assertSupportedGridValue(stepPercentMinimum, "Minimum step percent", true);
	return {
		allocationPerBuy,
		cycleProfitMaximum: maximum.profit,
		cycleProfitMaximumPercent: maximum.profitPercent,
		cycleProfitMinimum: minimum.profit,
		cycleProfitMinimumPercent: minimum.profitPercent,
		stepPercentMaximum,
		stepPercentMinimum,
	};
}
