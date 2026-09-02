import Decimal from "decimal.js";

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
		decimal = new Decimal(value);
	} catch {
		throw new RangeError(`${name} must be a positive finite number`);
	}

	if (!decimal.isFinite() || !decimal.gt(0)) {
		throw new RangeError(`${name} must be a positive finite number`);
	}

	return decimal;
}

export function assertMaintenanceMarginRatio(value: number): void {
	if (!Number.isFinite(value) || value < 0 || value >= 1) {
		throw new RangeError(
			"Maintenance-margin ratio must be a finite number from 0 up to 1",
		);
	}
}
