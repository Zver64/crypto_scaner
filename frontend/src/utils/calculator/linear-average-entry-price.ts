import type { LinearAverageEntryPriceFill } from "./types";
import { assertPositiveFiniteNumber } from "./validation";

/**
 * Calculates the unrounded base-quantity-weighted entry price for spot or
 * USD(S)-M fills.
 */
export function calculateLinearAverageEntryPrice(
	fills: readonly LinearAverageEntryPriceFill[],
): number {
	if (fills.length === 0) {
		throw new RangeError("At least one fill is required");
	}

	let totalQuantity = 0;
	let totalQuoteCost = 0;

	for (const { quantity, price } of [...fills].sort(compareLinearFills)) {
		assertPositiveFiniteNumber(quantity, "Quantity");
		assertPositiveFiniteNumber(price, "Price");
		totalQuantity += quantity;
		totalQuoteCost += quantity * price;
	}

	assertPositiveFiniteNumber(totalQuantity, "Total quantity");
	assertPositiveFiniteNumber(totalQuoteCost, "Total quote cost");
	return totalQuoteCost / totalQuantity;
}

function compareLinearFills(
	left: LinearAverageEntryPriceFill,
	right: LinearAverageEntryPriceFill,
): number {
	return left.quantity - right.quantity || left.price - right.price;
}
