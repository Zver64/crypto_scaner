export type PositionDirection = "long" | "short";

export interface UsdmIsolatedLiquidationPriceInput {
	direction: PositionDirection;
	entryPrice: number;
	leverage: number;
	maintenanceMarginRatio: number;
	quantity: number;
}

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
	if (!Number.isFinite(entryPrice) || entryPrice <= 0) {
		throw new RangeError("Entry Price must be a positive finite number");
	}
	if (!Number.isFinite(leverage) || leverage <= 0) {
		throw new RangeError("Leverage must be a positive finite number");
	}
	if (!Number.isFinite(quantity) || quantity <= 0) {
		throw new RangeError("Quantity must be a positive finite number");
	}
	if (
		!Number.isFinite(maintenanceMarginRatio) ||
		maintenanceMarginRatio < 0 ||
		maintenanceMarginRatio >= 1
	) {
		throw new RangeError(
			"Maintenance-margin ratio must be a finite number from 0 up to 1",
		);
	}

	if (direction === "long") {
		return (entryPrice * (1 - 1 / leverage)) / (1 - maintenanceMarginRatio);
	}

	return (entryPrice * (1 + 1 / leverage)) / (1 + maintenanceMarginRatio);
}
