import type Decimal from "decimal.js";
import type { CoinmIsolatedLiquidationPriceOptions } from "@/utils/calculator/types";
import {
	assertPositiveFiniteNumber,
	CalculatorDecimal,
	maintenanceMarginRatioDecimal,
	positiveFiniteDecimal,
} from "@/utils/calculator/validation";

/**
 * Estimates the unrounded liquidation price for a single isolated COIN-M Position.
 * The estimate uses one maintenance-margin ratio and intentionally omits Binance
 * risk-bracket adjustments, so it is conservative beyond the first bracket.
 */
export function calculateCoinmIsolatedLiquidationPrice({
	direction,
	contractCount,
	contractSize: contractSizeValue,
	entryPrice: entryPriceValue,
	isolatedWalletBalance: isolatedWalletBalanceValue,
	maintenanceMarginRatio: maintenanceMarginRatioValue,
}: CoinmIsolatedLiquidationPriceOptions): Decimal {
	assertPositiveFiniteNumber(contractCount, "Contract count");
	const contractSize = new CalculatorDecimal(
		positiveFiniteDecimal(contractSizeValue, "Contract size"),
	);
	const entryPrice = new CalculatorDecimal(
		positiveFiniteDecimal(entryPriceValue, "Entry Price"),
	);
	const isolatedWalletBalance = new CalculatorDecimal(
		positiveFiniteDecimal(
			isolatedWalletBalanceValue,
			"Isolated wallet balance",
		),
	);
	const maintenanceMarginRatio = new CalculatorDecimal(
		maintenanceMarginRatioDecimal(maintenanceMarginRatioValue),
	);
	const positionNotional = contractSize.times(contractCount);
	if (!positionNotional.isFinite() || !positionNotional.gt(0)) {
		throw new RangeError("Position notional is outside the supported range");
	}

	const entryBaseAssetValue = positionNotional.div(entryPrice);
	if (!entryBaseAssetValue.isFinite() || !entryBaseAssetValue.gt(0)) {
		throw new RangeError(
			"Entry base-asset value is outside the supported range",
		);
	}

	const one = new CalculatorDecimal(1);
	let liquidationPrice: Decimal;
	if (direction === "long") {
		const marginAdjustment = one.plus(maintenanceMarginRatio);
		const adjustedWalletBalance = isolatedWalletBalance.div(marginAdjustment);
		const adjustedEntryBaseAssetValue =
			entryBaseAssetValue.div(marginAdjustment);
		if (!adjustedWalletBalance.isFinite() || !adjustedWalletBalance.gt(0)) {
			throw new RangeError(
				"Adjusted wallet balance is outside the supported range",
			);
		}
		if (
			!adjustedEntryBaseAssetValue.isFinite() ||
			!adjustedEntryBaseAssetValue.gt(0)
		) {
			throw new RangeError(
				"Adjusted entry base-asset value is outside the supported range",
			);
		}
		const denominator = adjustedWalletBalance.plus(adjustedEntryBaseAssetValue);
		if (!denominator.isFinite() || !denominator.gt(0)) {
			throw new RangeError(
				"Liquidation-price denominator is outside the supported range",
			);
		}
		liquidationPrice = positionNotional.div(denominator);
	} else {
		const numerator = one.minus(maintenanceMarginRatio).times(positionNotional);
		if (!numerator.isFinite() || !numerator.gt(0)) {
			throw new RangeError(
				"Liquidation-price numerator is outside the supported range",
			);
		}
		const denominator = entryBaseAssetValue.minus(isolatedWalletBalance);
		if (!denominator.isFinite() || denominator.isZero()) {
			throw new RangeError(
				"Liquidation-price denominator is outside the supported range",
			);
		}
		liquidationPrice = numerator.div(denominator);
	}

	if (!liquidationPrice.isFinite() || liquidationPrice.isZero()) {
		throw new RangeError("Liquidation price is outside the supported range");
	}
	return liquidationPrice;
}
