import Decimal from "decimal.js";

export const CalculatorDecimal = Decimal.clone({ precision: 60 });

export function positiveFiniteDecimal(
	value: Decimal.Value,
	name: string,
): Decimal {
	let decimal: Decimal;
	try {
		decimal = new CalculatorDecimal(value);
	} catch {
		throw new RangeError(`${name} must be a positive finite decimal`);
	}

	if (!decimal.isFinite() || !decimal.gt(0)) {
		throw new RangeError(`${name} must be a positive finite decimal`);
	}

	return decimal;
}
