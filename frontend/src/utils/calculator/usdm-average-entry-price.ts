import type Decimal from "decimal.js";
import type { UsdmAverageEntryPriceFill } from "@/utils/calculator/types";
import {
	CalculatorDecimal,
	positiveFiniteDecimal,
} from "@/utils/calculator/validation";

/**
 * Calculates the unrounded USD(S)-M average Entry Price from quote-notional
 * entries.
 */
export function calculateUsdmAverageEntryPrice(
	entries: readonly UsdmAverageEntryPriceFill[],
): Decimal {
	if (entries.length === 0) {
		throw new RangeError("At least one entry is required");
	}

	let totalQuoteNotional = new CalculatorDecimal(0);
	let totalBaseQuantity = new CalculatorDecimal(0);

	for (const entry of entries) {
		const price = new CalculatorDecimal(
			positiveFiniteDecimal(entry.price, "Entry Price"),
		);
		const quoteNotional = new CalculatorDecimal(
			positiveFiniteDecimal(entry.quoteNotional, "Quote notional"),
		);
		totalQuoteNotional = totalQuoteNotional.plus(quoteNotional);
		totalBaseQuantity = totalBaseQuantity.plus(quoteNotional.div(price));
	}

	const average = totalQuoteNotional.div(totalBaseQuantity);
	if (!average.isFinite() || !average.gt(0)) {
		throw new RangeError("Average entry price is outside the supported range");
	}
	return average;
}
