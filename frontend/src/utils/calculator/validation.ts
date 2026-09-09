import Decimal from "decimal.js";

export const CalculatorDecimal = Decimal.clone({ precision: 60 });

export function assertPositiveFiniteNumber(value: number, name: string): void {
	if (!Number.isFinite(value) || value <= 0) {
		throw new RangeError(`${name} must be a positive finite number`);
	}
}

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

export function maintenanceMarginRatioDecimal(value: Decimal.Value): Decimal {
	let ratio: Decimal;
	try {
		ratio = new CalculatorDecimal(value);
	} catch {
		throw new RangeError(
			"Maintenance-margin ratio must be a finite decimal from 0 up to 1",
		);
	}

	if (!ratio.isFinite() || ratio.lt(0) || ratio.gte(1)) {
		throw new RangeError(
			"Maintenance-margin ratio must be a finite decimal from 0 up to 1",
		);
	}
	return ratio;
}
