import type Decimal from "decimal.js";
import type { UsdmIsolatedLiquidationPriceInput } from "@/utils/calculator/types";
import {
	CalculatorDecimal,
	maintenanceMarginRatioDecimal,
	positiveFiniteDecimal,
} from "@/utils/calculator/validation";

/**
 * Estimates the unrounded liquidation price for an isolated USD(S)-M Position.
 * The estimate uses one maintenance-margin ratio and intentionally omits
 * Binance risk-bracket adjustments, making it conservative for later brackets.
 */
export function calculateUsdmIsolatedLiquidationPrice({
	direction,
	entryPrice: entryPriceValue,
	leverage: leverageValue,
	maintenanceMarginRatio: maintenanceMarginRatioValue,
	quantity: quantityValue,
}: UsdmIsolatedLiquidationPriceInput): Decimal {
	const entryPrice = new CalculatorDecimal(
		positiveFiniteDecimal(entryPriceValue, "Entry Price"),
	);
	const leverage = new CalculatorDecimal(
		positiveFiniteDecimal(leverageValue, "Leverage"),
	);
	positiveFiniteDecimal(quantityValue, "Quantity");
	const maintenanceMarginRatio = new CalculatorDecimal(
		maintenanceMarginRatioDecimal(maintenanceMarginRatioValue),
	);
	const one = new CalculatorDecimal(1);
	const inverseLeverage = one.div(leverage);
	if (!inverseLeverage.isFinite() || inverseLeverage.isZero()) {
		throw new RangeError("Inverse leverage is outside the supported range");
	}

	const leverageAdjustment =
		direction === "long"
			? one.minus(inverseLeverage)
			: one.plus(inverseLeverage);
	if (
		!leverageAdjustment.isFinite() ||
		(leverageAdjustment.isZero() && !leverage.eq(1))
	) {
		throw new RangeError("Leverage adjustment is outside the supported range");
	}

	const numerator = entryPrice.times(leverageAdjustment);
	if (
		!numerator.isFinite() ||
		(numerator.isZero() && !leverageAdjustment.isZero())
	) {
		throw new RangeError(
			"Liquidation-price numerator is outside the supported range",
		);
	}

	const marginAdjustment =
		direction === "long"
			? one.minus(maintenanceMarginRatio)
			: one.plus(maintenanceMarginRatio);
	if (!marginAdjustment.isFinite() || !marginAdjustment.gt(0)) {
		throw new RangeError("Margin adjustment is outside the supported range");
	}

	const liquidationPrice = numerator.div(marginAdjustment);
	if (
		!liquidationPrice.isFinite() ||
		(liquidationPrice.isZero() && !numerator.isZero())
	) {
		throw new RangeError("Liquidation price is outside the supported range");
	}
	return liquidationPrice;
}
