import type { UsdmIsolatedLiquidationPriceInput } from "./types";
import {
	assertMaintenanceMarginRatio,
	assertPositiveFiniteNumber,
} from "./validation";

/**
 * Estimates the unrounded liquidation price for an isolated USD(S)-M Position.
 * The estimate uses one maintenance-margin ratio and intentionally omits
 * Binance risk-bracket adjustments, making it conservative for later brackets.
 */
export function calculateUsdmIsolatedLiquidationPrice({
	direction,
	entryPrice,
	leverage,
	maintenanceMarginRatio,
	quantity,
}: UsdmIsolatedLiquidationPriceInput): number {
	assertPositiveFiniteNumber(entryPrice, "Entry Price");
	assertPositiveFiniteNumber(leverage, "Leverage");
	assertPositiveFiniteNumber(quantity, "Quantity");
	assertMaintenanceMarginRatio(maintenanceMarginRatio);

	if (direction === "long") {
		return (entryPrice * (1 - 1 / leverage)) / (1 - maintenanceMarginRatio);
	}

	return (entryPrice * (1 + 1 / leverage)) / (1 + maintenanceMarginRatio);
}
