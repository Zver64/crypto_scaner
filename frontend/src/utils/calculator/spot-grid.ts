import type Decimal from "decimal.js";
import {
	assertSupportedGridValue,
	GridDecimal,
	gridLevels,
	parseGridInput,
} from "@/utils/calculator/grid";
import type { GridInput, GridType } from "@/utils/calculator/types";

const SPOT_GRID_FEE_RATE = new GridDecimal("0.001");
const SPOT_GRID_ONE_MINUS_FEE = new GridDecimal(1).minus(SPOT_GRID_FEE_RATE);

export interface SpotGrid {
	allocationPerBuy: Decimal;
	gridCount: number;
	levels: Decimal[];
	lowerPrice: Decimal;
	upperPrice: Decimal;
}

/**
 * Parses a spot grid with equal quote allocations: a buy order on every level
 * but the upper one, which is a sell level. Rejects a grid whose buy orders
 * would get a quantity outside the supported range.
 */
export function spotGrid(input: GridInput, gridType: GridType): SpotGrid {
	const { gridCount, investment, lowerPrice, upperPrice } =
		parseGridInput(input);
	const levels = gridLevels(lowerPrice, upperPrice, gridCount, gridType);
	const allocationPerBuy = investment.div(gridCount);
	assertSupportedGridValue(allocationPerBuy, "Allocation per buy", true);
	for (const buyPrice of levels.slice(0, -1)) {
		const netQuantity = allocationPerBuy
			.div(buyPrice)
			.times(SPOT_GRID_ONE_MINUS_FEE);
		assertSupportedGridValue(netQuantity, "Net buy quantity", true);
	}
	return { allocationPerBuy, gridCount, levels, lowerPrice, upperPrice };
}

export interface SpotGridCycle {
	profit: Decimal;
	profitPercent: Decimal;
}

/**
 * Returns the net result of buying `allocationPerBuy` at one level and selling
 * the bought quantity at a level `sellToBuyRatio` higher, with the fee charged
 * on both legs.
 */
export function spotGridCycle(
	allocationPerBuy: Decimal,
	sellToBuyRatio: Decimal,
): SpotGridCycle {
	const cycleReturn = sellToBuyRatio
		.times(SPOT_GRID_ONE_MINUS_FEE.pow(2))
		.minus(1);
	const profit = allocationPerBuy.times(cycleReturn);
	const profitPercent = cycleReturn.times(100);
	assertSupportedGridValue(profit, "Cycle profit");
	assertSupportedGridValue(profitPercent, "Cycle return percent");
	return { profit, profitPercent };
}
