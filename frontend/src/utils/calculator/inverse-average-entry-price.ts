import type Decimal from "decimal.js";
import type { InverseAverageEntryPriceFill } from "@/utils/calculator/types";
import {
	assertPositiveFiniteNumber,
	CalculatorDecimal,
	positiveFiniteDecimal,
} from "@/utils/calculator/validation";

/**
 * Calculates the unrounded contract-notional-weighted entry price for COIN-M
 * fills. COIN-M contract notional is fixed in quote currency, so each fill's
 * base quantity is its contract notional divided by its execution price.
 */
export function calculateInverseAverageEntryPrice(
	fills: readonly InverseAverageEntryPriceFill[],
): Decimal {
	if (fills.length === 0) {
		throw new RangeError("At least one fill is required");
	}

	let totalContractNotional = new CalculatorDecimal(0);
	let totalBaseQuantity = new CalculatorDecimal(0);
	let direction: InverseAverageEntryPriceFill["direction"];

	for (const fill of fills) {
		assertPositiveFiniteNumber(fill.contractCount, "Contract count");
		const contractSize = new CalculatorDecimal(
			positiveFiniteDecimal(fill.contractSize, "Contract size"),
		);
		const price = new CalculatorDecimal(
			positiveFiniteDecimal(fill.price, "Price"),
		);

		if (direction && fill.direction && direction !== fill.direction) {
			throw new RangeError("All fills must have the same position direction");
		}
		direction ??= fill.direction;

		const contractNotional = contractSize.times(fill.contractCount);
		totalContractNotional = totalContractNotional.plus(contractNotional);
		totalBaseQuantity = totalBaseQuantity.plus(contractNotional.div(price));
	}

	const average = totalContractNotional.div(totalBaseQuantity);
	if (!average.isFinite() || !average.gt(0)) {
		throw new RangeError("Average entry price is outside the supported range");
	}
	return average;
}
