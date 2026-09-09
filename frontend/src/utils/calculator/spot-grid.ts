import Decimal from "decimal.js";

export const SpotGridDecimal = Decimal.clone({
	precision: 60,
	maxE: 1000,
	minE: -1000,
	toExpNeg: -6,
	toExpPos: 9,
});

export const SPOT_GRID_FEE_RATE = new SpotGridDecimal("0.001");
export const SPOT_GRID_ONE_MINUS_FEE = new SpotGridDecimal(1).minus(
	SPOT_GRID_FEE_RATE,
);
export const SPOT_GRID_MAX_COUNT = 1000;

const MAX_INPUT_LENGTH = 80;
const MAX_SIGNIFICANT_DIGITS = 50;
const DECIMAL_INPUT = /^\+?(?:\d+(?:\.\d*)?|\.\d+)(?:e[+-]?\d+)?$/i;

export interface SpotGridInput {
	gridCount: string;
	investment: string;
	lowerPrice: string;
	upperPrice: string;
}

export function parseSpotGridDecimal(value: string, name: string): Decimal {
	if (value.length > MAX_INPUT_LENGTH || !DECIMAL_INPUT.test(value)) {
		throw new RangeError(`${name} must be a valid positive decimal`);
	}

	const exponentText = value.match(/e([+-]?\d+)$/i)?.[1];
	if (exponentText && Math.abs(Number(exponentText)) > 1000) {
		throw new RangeError(`${name} exponent is outside the supported range`);
	}

	const coefficient = value.split(/e/i, 1)[0];
	const significantDigits = coefficient
		.replace(/^\+/, "")
		.replace(".", "")
		.replace(/^0+/, "").length;
	if (significantDigits > MAX_SIGNIFICANT_DIGITS) {
		throw new RangeError(`${name} has too many significant digits`);
	}

	const decimal = new SpotGridDecimal(value);
	if (!decimal.isFinite() || !decimal.gt(0)) {
		throw new RangeError(`${name} is outside the supported range`);
	}
	return decimal;
}

export function parseSpotGridCount(value: string): number {
	if (!/^\d+$/.test(value) || value.length > 4) {
		throw new RangeError("Grid count must be a whole number");
	}
	const count = Number(value);
	if (count < 1 || count > SPOT_GRID_MAX_COUNT) {
		throw new RangeError(`Grid count must be from 1 to ${SPOT_GRID_MAX_COUNT}`);
	}
	return count;
}

export function assertSupportedSpotGridValue(
	value: Decimal,
	name: string,
	mustBePositive = false,
): void {
	if (!value.isFinite() || (mustBePositive && !value.gt(0))) {
		throw new RangeError(`${name} is outside the supported range`);
	}
}
