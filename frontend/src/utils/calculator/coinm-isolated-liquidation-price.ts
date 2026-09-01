import type { CoinmIsolatedLiquidationPriceOptions } from "./types";
import {
	assertMaintenanceMarginRatio,
	assertPositiveFiniteNumber,
} from "./validation";

/**
 * Estimates the unrounded liquidation price for a single isolated COIN-M Position.
 * The estimate uses one maintenance-margin ratio and intentionally omits Binance
 * risk-bracket adjustments, so it is conservative beyond the first bracket.
 */
export function calculateCoinmIsolatedLiquidationPrice({
	direction,
	contractCount,
	contractSize,
	entryPrice,
	isolatedWalletBalance,
	maintenanceMarginRatio,
}: CoinmIsolatedLiquidationPriceOptions): number {
	assertPositiveFiniteNumber(contractCount, "Contract count");
	assertPositiveFiniteNumber(contractSize, "Contract size");
	assertPositiveFiniteNumber(entryPrice, "Entry Price");
	assertPositiveFiniteNumber(isolatedWalletBalance, "Isolated wallet balance");
	assertMaintenanceMarginRatio(maintenanceMarginRatio);

	const positionNotional = contractCount * contractSize;
	const entryBaseAssetValue = positionNotional / entryPrice;

	if (direction === "long") {
		return (
			positionNotional /
			(isolatedWalletBalance / (1 + maintenanceMarginRatio) +
				entryBaseAssetValue / (1 + maintenanceMarginRatio))
		);
	}

	return (
		((1 - maintenanceMarginRatio) * positionNotional) /
		(entryBaseAssetValue - isolatedWalletBalance)
	);
}
