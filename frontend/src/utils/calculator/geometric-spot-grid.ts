import type Decimal from "decimal.js";
import { calculateLinearAverageEntryPrice } from "@/utils/calculator/linear-average-entry-price";
import {
	assertSupportedSpotGridValue,
	parseSpotGridCount,
	parseSpotGridDecimal,
	SPOT_GRID_ONE_MINUS_FEE,
	SpotGridDecimal,
	type SpotGridInput,
} from "@/utils/calculator/spot-grid";
import type { LinearAverageEntryPriceFill } from "@/utils/calculator/types";

export type GeometricSpotGridInput = SpotGridInput;

export interface GeometricSpotGridEstimate {
	allocationPerBuy: Decimal;
	averageEntryPrice: Decimal;
	cycleProfit: Decimal;
	cycleProfitPercent: Decimal;
	gridCount: number;
	levelCount: number;
	stepPercent: Decimal;
	stepRatio: Decimal;
	totalNetQuantity: Decimal;
}

/**
 * Estimates a geometric Binance spot grid using equal percentage spacing and
 * equal quote allocations. The upper level is a sell level, not a buy level.
 * All returned values are unrounded Decimal instances.
 */
export function calculateGeometricSpotGrid(
	input: GeometricSpotGridInput,
): GeometricSpotGridEstimate {
	const lowerPrice = parseSpotGridDecimal(input.lowerPrice, "Lower price");
	const upperPrice = parseSpotGridDecimal(input.upperPrice, "Upper price");
	const investment = parseSpotGridDecimal(input.investment, "Investment");
	const gridCount = parseSpotGridCount(input.gridCount);

	if (!upperPrice.gt(lowerPrice)) {
		throw new RangeError("Upper price must be greater than lower price");
	}

	const stepRatio = upperPrice
		.div(lowerPrice)
		.pow(new SpotGridDecimal(1).div(gridCount));
	if (!stepRatio.gt(1) || lowerPrice.times(stepRatio).eq(lowerPrice)) {
		throw new RangeError(
			"Price range is too narrow at the supported precision",
		);
	}

	const allocationPerBuy = investment.div(gridCount);
	assertSupportedSpotGridValue(allocationPerBuy, "Allocation per buy", true);

	const averagePriceFills: LinearAverageEntryPriceFill[] = [];
	let totalNetQuantity = new SpotGridDecimal(0);
	let previousBuyPrice: Decimal | undefined;

	for (let index = 0; index < gridCount; index += 1) {
		const buyPrice = lowerPrice.times(stepRatio.pow(index));
		if (previousBuyPrice?.eq(buyPrice)) {
			throw new RangeError(
				"Derived grid levels collapse at the supported precision",
			);
		}
		previousBuyPrice = buyPrice;

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
	}

	const cycleReturn = stepRatio.times(SPOT_GRID_ONE_MINUS_FEE.pow(2)).minus(1);
	const cycleProfit = allocationPerBuy.times(cycleReturn);
	const cycleProfitPercent = cycleReturn.times(100);
	const stepPercent = stepRatio.minus(1).times(100);
	const averageEntryPrice = calculateLinearAverageEntryPrice(averagePriceFills);

	assertSupportedSpotGridValue(cycleProfit, "Cycle profit");
	assertSupportedSpotGridValue(cycleProfitPercent, "Cycle return percent");
	assertSupportedSpotGridValue(stepPercent, "Step percent", true);
	assertSupportedSpotGridValue(averageEntryPrice, "Average entry price", true);

	return {
		allocationPerBuy,
		averageEntryPrice,
		cycleProfit,
		cycleProfitPercent,
		gridCount,
		levelCount: gridCount + 1,
		stepPercent,
		stepRatio,
		totalNetQuantity,
	};
}
