import Decimal from "decimal.js";
import { calculateLinearAverageEntryPrice } from "@/utils/calculator/linear-average-entry-price";
import type { LinearAverageEntryPriceFill } from "@/utils/calculator/types";
import { positiveFiniteDecimal } from "@/utils/calculator/validation";

const SpotGridDecimal = Decimal.clone({
	precision: 60,
	maxE: 1000,
	minE: -1000,
	toExpNeg: -6,
	toExpPos: 9,
});

const FEE_RATE = new SpotGridDecimal("0.001");
const ONE_MINUS_FEE = new SpotGridDecimal(1).minus(FEE_RATE);
const MAX_INPUT_LENGTH = 80;
const MAX_SIGNIFICANT_DIGITS = 50;
const DECIMAL_INPUT = /^\+?(?:\d+(?:\.\d*)?|\.\d+)(?:e[+-]?\d+)?$/i;

export const ARITHMETIC_SPOT_GRID_MAX_COUNT = 1000;

export interface ArithmeticSpotGridInput {
	gridCount: string;
	investment: string;
	lowerPrice: string;
	upperPrice: string;
}

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

function parseBoundedPositiveDecimal(value: string, name: string): Decimal {
	if (value.length > MAX_INPUT_LENGTH || !DECIMAL_INPUT.test(value)) {
		throw new RangeError(`${name} must be a valid positive decimal`);
	}

	const exponentText = value.match(/e([+-]?\d+)$/i)?.[1];
	if (exponentText && Math.abs(Number(exponentText)) > 1000) {
		throw new RangeError(`${name} exponent is outside the supported range`);
	}

	const coefficient = value.split(/e/i, 1)[0];
	const significantDigits = coefficient
		.replace(/^\+/, "")
		.replace(".", "")
		.replace(/^0+/, "").length;
	if (significantDigits > MAX_SIGNIFICANT_DIGITS) {
		throw new RangeError(`${name} has too many significant digits`);
	}

	const decimal = new SpotGridDecimal(positiveFiniteDecimal(value, name));
	if (!decimal.isFinite() || !decimal.gt(0)) {
		throw new RangeError(`${name} is outside the supported range`);
	}
	return decimal;
}

function parseGridCount(value: string): number {
	if (!/^\d+$/.test(value) || value.length > 4) {
		throw new RangeError("Grid count must be a whole number");
	}
	const count = Number(value);
	if (count < 1 || count > ARITHMETIC_SPOT_GRID_MAX_COUNT) {
		throw new RangeError(
			`Grid count must be from 1 to ${ARITHMETIC_SPOT_GRID_MAX_COUNT}`,
		);
	}
	return count;
}

function assertSupportedDerivedValue(
	value: Decimal,
	name: string,
	mustBePositive = false,
): void {
	if (!value.isFinite() || (mustBePositive && !value.gt(0))) {
		throw new RangeError(`${name} is outside the supported range`);
	}
}

/**
 * Estimates an arithmetic Binance spot grid using equal quote allocations.
 * All returned values are unrounded Decimal instances.
 */
export function calculateArithmeticSpotGrid(
	input: ArithmeticSpotGridInput,
): ArithmeticSpotGridEstimate {
	const lowerPrice = parseBoundedPositiveDecimal(
		input.lowerPrice,
		"Lower price",
	);
	const upperPrice = parseBoundedPositiveDecimal(
		input.upperPrice,
		"Upper price",
	);
	const investment = parseBoundedPositiveDecimal(
		input.investment,
		"Investment",
	);
	const gridCount = parseGridCount(input.gridCount);

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
	assertSupportedDerivedValue(allocationPerBuy, "Allocation per buy", true);
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
		const netQuantity = allocationPerBuy.div(buyPrice).times(ONE_MINUS_FEE);
		assertSupportedDerivedValue(netQuantity, "Net buy quantity", true);
		totalNetQuantity = totalNetQuantity.plus(netQuantity);
		assertSupportedDerivedValue(totalNetQuantity, "Total net quantity", true);
		const effectiveBuyPrice = buyPrice.div(ONE_MINUS_FEE);
		assertSupportedDerivedValue(effectiveBuyPrice, "Effective buy price", true);
		averagePriceFills.push({ quantity: netQuantity, price: effectiveBuyPrice });

		const cycleReturn = stepPrice
			.div(buyPrice)
			.times(ONE_MINUS_FEE.pow(2))
			.minus(FEE_RATE.times(new SpotGridDecimal(2).minus(FEE_RATE)));
		const cycleProfit = allocationPerBuy.times(cycleReturn);
		const cyclePercent = cycleReturn.times(100);
		assertSupportedDerivedValue(cycleReturn, "Cycle return");
		assertSupportedDerivedValue(cycleProfit, "Cycle profit");
		assertSupportedDerivedValue(cyclePercent, "Cycle return percent");
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
	assertSupportedDerivedValue(averageEntryPrice, "Average entry price", true);
	assertSupportedDerivedValue(stepPercentMaximum, "Maximum step percent", true);
	assertSupportedDerivedValue(stepPercentMinimum, "Minimum step percent", true);
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
