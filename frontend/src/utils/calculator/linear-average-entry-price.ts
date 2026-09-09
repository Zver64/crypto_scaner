import type Decimal from "decimal.js";
import type { LinearAverageEntryPriceFill } from "@/utils/calculator/types";
import {
	CalculatorDecimal,
	positiveFiniteDecimal,
} from "@/utils/calculator/validation";

/**
 * Calculates the unrounded base-quantity-weighted entry price for spot or
 * USD(S)-M fills.
 */
export function calculateLinearAverageEntryPrice(
	fills: readonly LinearAverageEntryPriceFill[],
): Decimal {
	if (fills.length === 0) {
		throw new RangeError("At least one fill is required");
	}

	const parsedFills = fills
		.map((fill) => ({
			quantity: new CalculatorDecimal(
				positiveFiniteDecimal(fill.quantity, "Quantity"),
			),
			price: new CalculatorDecimal(positiveFiniteDecimal(fill.price, "Price")),
		}))
		.sort(
			(left, right) =>
				left.quantity.comparedTo(right.quantity) ||
				left.price.comparedTo(right.price),
		);
	let totalQuantity = new CalculatorDecimal(0);
	let totalQuoteCost = new CalculatorDecimal(0);

	for (const { quantity, price } of parsedFills) {
		totalQuantity = totalQuantity.plus(quantity);
		totalQuoteCost = totalQuoteCost.plus(quantity.times(price));
	}

	const average = totalQuoteCost.div(totalQuantity);
	if (!average.isFinite() || !average.gt(0)) {
		throw new RangeError("Average entry price is outside the supported range");
	}
	return average;
}
