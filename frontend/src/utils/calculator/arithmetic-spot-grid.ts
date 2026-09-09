import type Decimal from "decimal.js";
import { calculateLinearAverageEntryPrice } from "@/utils/calculator/linear-average-entry-price";
import {
	assertSupportedSpotGridValue,
	parseSpotGridCount,
	parseSpotGridDecimal,
	SPOT_GRID_FEE_RATE,
	SPOT_GRID_MAX_COUNT,
	SPOT_GRID_ONE_MINUS_FEE,
	SpotGridDecimal,
	type SpotGridInput,
} from "@/utils/calculator/spot-grid";
import type { LinearAverageEntryPriceFill } from "@/utils/calculator/types";

export const ARITHMETIC_SPOT_GRID_MAX_COUNT = SPOT_GRID_MAX_COUNT;
export type ArithmeticSpotGridInput = SpotGridInput;

export interface ArithmeticSpotGridEstimate {
	allocationPerBuy: Decimal;
	averageEntryPrice: Decimal;
	cycleProfitMaximum: Decimal;
	cycleProfitMaximumPercent: Decimal;
	cycleProfitMinimum: Decimal;
	cycleProfitMinimumPercent: Decimal;
	gridCount: number;
	levelCount: number;
	stepPercentMaximum: Decimal;
	stepPercentMinimum: Decimal;
	stepPrice: Decimal;
	totalNetQuantity: Decimal;
}

/**
 * Estimates an arithmetic Binance spot grid using equal quote allocations.
 * All returned values are unrounded Decimal instances.
 */
export function calculateArithmeticSpotGrid(
	input: ArithmeticSpotGridInput,
): ArithmeticSpotGridEstimate {
	const lowerPrice = parseSpotGridDecimal(input.lowerPrice, "Lower price");
	const upperPrice = parseSpotGridDecimal(input.upperPrice, "Upper price");
	const investment = parseSpotGridDecimal(input.investment, "Investment");
	const gridCount = parseSpotGridCount(input.gridCount);

	if (!upperPrice.gt(lowerPrice)) {
		throw new RangeError("Upper price must be greater than lower price");
	}

	const stepPrice = upperPrice.minus(lowerPrice).div(gridCount);
	if (!stepPrice.gt(0) || lowerPrice.plus(stepPrice).eq(lowerPrice)) {
		throw new RangeError(
			"Price range is too narrow at the supported precision",
		);
	}

	const allocationPerBuy = investment.div(gridCount);
	assertSupportedSpotGridValue(allocationPerBuy, "Allocation per buy", true);
	const averagePriceFills: LinearAverageEntryPriceFill[] = [];
	let totalNetQuantity = new SpotGridDecimal(0);
	let cycleProfitMinimum: Decimal | undefined;
	let cycleProfitMaximum: Decimal | undefined;
	let cycleProfitMinimumPercent: Decimal | undefined;
	let cycleProfitMaximumPercent: Decimal | undefined;

	for (let index = 0; index < gridCount; index += 1) {
		const buyPrice = lowerPrice.plus(stepPrice.times(index));
		if (index > 0 && buyPrice.eq(lowerPrice)) {
			throw new RangeError(
				"Derived grid levels collapse at the supported precision",
			);
		}
		const netQuantity = allocationPerBuy
			.div(buyPrice)
			.times(SPOT_GRID_ONE_MINUS_FEE);
		assertSupportedSpotGridValue(netQuantity, "Net buy quantity", true);
		totalNetQuantity = totalNetQuantity.plus(netQuantity);
		assertSupportedSpotGridValue(totalNetQuantity, "Total net quantity", true);
		const effectiveBuyPrice = buyPrice.div(SPOT_GRID_ONE_MINUS_FEE);
		assertSupportedSpotGridValue(
			effectiveBuyPrice,
			"Effective buy price",
			true,
		);
		averagePriceFills.push({ quantity: netQuantity, price: effectiveBuyPrice });

		const cycleReturn = stepPrice
			.div(buyPrice)
			.times(SPOT_GRID_ONE_MINUS_FEE.pow(2))
			.minus(
				SPOT_GRID_FEE_RATE.times(
					new SpotGridDecimal(2).minus(SPOT_GRID_FEE_RATE),
				),
			);
		const cycleProfit = allocationPerBuy.times(cycleReturn);
		const cyclePercent = cycleReturn.times(100);
		assertSupportedSpotGridValue(cycleReturn, "Cycle return");
		assertSupportedSpotGridValue(cycleProfit, "Cycle profit");
		assertSupportedSpotGridValue(cyclePercent, "Cycle return percent");
		if (
			cycleProfitMinimum === undefined ||
			cycleProfit.lt(cycleProfitMinimum)
		) {
			cycleProfitMinimum = cycleProfit;
			cycleProfitMinimumPercent = cyclePercent;
		}
		if (
			cycleProfitMaximum === undefined ||
			cycleProfit.gt(cycleProfitMaximum)
		) {
			cycleProfitMaximum = cycleProfit;
			cycleProfitMaximumPercent = cyclePercent;
		}
	}

	const highestBuyPrice = upperPrice.minus(stepPrice);
	const averageEntryPrice = calculateLinearAverageEntryPrice(averagePriceFills);
	const stepPercentMaximum = stepPrice.div(lowerPrice).times(100);
	const stepPercentMinimum = stepPrice.div(highestBuyPrice).times(100);
	assertSupportedSpotGridValue(averageEntryPrice, "Average entry price", true);
	assertSupportedSpotGridValue(
		stepPercentMaximum,
		"Maximum step percent",
		true,
	);
	assertSupportedSpotGridValue(
		stepPercentMinimum,
		"Minimum step percent",
		true,
	);
	return {
		allocationPerBuy,
		averageEntryPrice,
		cycleProfitMaximum: cycleProfitMaximum as Decimal,
		cycleProfitMaximumPercent: cycleProfitMaximumPercent as Decimal,
		cycleProfitMinimum: cycleProfitMinimum as Decimal,
		cycleProfitMinimumPercent: cycleProfitMinimumPercent as Decimal,
		gridCount,
		levelCount: gridCount + 1,
		stepPercentMaximum,
		stepPercentMinimum,
		stepPrice,
		totalNetQuantity,
	};
}
