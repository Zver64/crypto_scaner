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

	let totalContractNotional = 0;
	let totalBaseQuantity = 0;

	for (const { contractCount, contractSize, price } of [...fills].sort(
		compareInverseFills,
	)) {
		assertPositiveFiniteNumber(contractCount, "Contract count");
		assertPositiveFiniteNumber(contractSize, "Contract size");
		assertPositiveFiniteNumber(price, "Price");

		const contractNotional = contractCount * contractSize;
		totalContractNotional += contractNotional;
		totalBaseQuantity += contractNotional / price;
	}

	assertPositiveFiniteNumber(totalContractNotional, "Total contract notional");
	assertPositiveFiniteNumber(totalBaseQuantity, "Total base quantity");
	return totalContractNotional / totalBaseQuantity;
}

function compareInverseFills(
	left: InverseAverageEntryPriceFill,
	right: InverseAverageEntryPriceFill,
): number {
	return (
		left.contractCount - right.contractCount ||
		left.contractSize - right.contractSize ||
		left.price - right.price
	);
}
