import Decimal from "decimal.js";
import type { UsdmAverageEntryPriceFill } from "./types";
import { positiveFiniteDecimal } from "./validation";

const UsdmDecimal = Decimal.clone({ precision: 40 });

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

	let totalQuoteNotional = new UsdmDecimal(0);
	let totalBaseQuantity = new UsdmDecimal(0);

	for (const entry of entries) {
		const price = new UsdmDecimal(
			positiveFiniteDecimal(entry.price, "Entry Price"),
		);
		const quoteNotional = new UsdmDecimal(
			positiveFiniteDecimal(entry.quoteNotional, "Quote notional"),
		);
		totalQuoteNotional = totalQuoteNotional.plus(quoteNotional);
		totalBaseQuantity = totalBaseQuantity.plus(quoteNotional.div(price));
	}

	return totalQuoteNotional.div(totalBaseQuantity);
}
