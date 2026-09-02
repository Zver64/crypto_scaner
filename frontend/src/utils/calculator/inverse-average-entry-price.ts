import Decimal from "decimal.js";
import type { InverseAverageEntryPriceFill } from "./types";
import { assertPositiveFiniteNumber } from "./validation";

/**
 * Calculates the unrounded contract-notional-weighted entry price for COIN-M
 * fills. COIN-M contract notional is fixed in quote currency, so each fill's
 * base quantity is its contract notional divided by its execution price.
 */
export function calculateInverseAverageEntryPrice(
	fills: readonly InverseAverageEntryPriceFill[],
): number {
	if (fills.length === 0) {
		throw new RangeError("At least one fill is required");
	}

	let totalContractNotional = new Decimal(0);
	let totalBaseQuantity = new Decimal(0);
	let direction: InverseAverageEntryPriceFill["direction"];

	for (const {
		contractCount,
		contractSize,
		direction: fillDirection,
		price,
	} of fills) {
		assertPositiveFiniteNumber(contractCount, "Contract count");
		assertPositiveFiniteNumber(contractSize, "Contract size");
		assertPositiveFiniteNumber(price, "Price");

		if (direction && fillDirection && direction !== fillDirection) {
			throw new RangeError("All fills must have the same position direction");
		}
		direction ??= fillDirection;

		const contractNotional = new Decimal(contractCount).mul(contractSize);
		totalContractNotional = totalContractNotional.add(contractNotional);
		totalBaseQuantity = totalBaseQuantity.add(contractNotional.div(price));
	}

	return totalContractNotional.div(totalBaseQuantity).toNumber();
}
