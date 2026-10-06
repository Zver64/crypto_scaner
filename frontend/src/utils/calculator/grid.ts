import Decimal from "decimal.js";
import type { GridInput, GridType } from "@/utils/calculator/types";

export const GridDecimal = Decimal.clone({
	precision: 60,
	maxE: 1000,
	minE: -1000,
	toExpNeg: -6,
	toExpPos: 9,
});

export const GRID_MAX_COUNT = 1000;

const MAX_INPUT_LENGTH = 80;
const MAX_SIGNIFICANT_DIGITS = 50;
const DECIMAL_INPUT = /^\+?(?:\d+(?:\.\d*)?|\.\d+)(?:e[+-]?\d+)?$/i;

export function parseGridDecimal(value: string, name: string): Decimal {
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

	const decimal = new GridDecimal(value);
	if (!decimal.isFinite() || !decimal.gt(0)) {
		throw new RangeError(`${name} is outside the supported range`);
	}
	return decimal;
}

export function parseGridCount(value: string): number {
	if (!/^\d+$/.test(value) || value.length > 4) {
		throw new RangeError("Grid count must be a whole number");
	}
	const count = Number(value);
	if (count < 1 || count > GRID_MAX_COUNT) {
		throw new RangeError(`Grid count must be from 1 to ${GRID_MAX_COUNT}`);
	}
	return count;
}

export interface ParsedGridInput {
	gridCount: number;
	investment: Decimal;
	lowerPrice: Decimal;
	upperPrice: Decimal;
}

/** Parses and validates the input every grid calculator shares. */
export function parseGridInput(input: GridInput): ParsedGridInput {
	const lowerPrice = parseGridDecimal(input.lowerPrice, "Lower price");
	const upperPrice = parseGridDecimal(input.upperPrice, "Upper price");
	const investment = parseGridDecimal(input.investment, "Investment");
	const gridCount = parseGridCount(input.gridCount);
	if (!upperPrice.gt(lowerPrice)) {
		throw new RangeError("Upper price must be greater than lower price");
	}
	return { gridCount, investment, lowerPrice, upperPrice };
}

export function assertSupportedGridValue(
	value: Decimal,
	name: string,
	mustBePositive = false,
): void {
	if (!value.isFinite() || (mustBePositive && !value.gt(0))) {
		throw new RangeError(`${name} is outside the supported range`);
	}
}

/** Returns the ratio between neighbouring levels of a geometric grid. */
export function geometricStepRatio(
	lowerPrice: Decimal,
	upperPrice: Decimal,
	gridCount: number,
): Decimal {
	return upperPrice.div(lowerPrice).pow(new GridDecimal(1).div(gridCount));
}

/**
 * Returns the `gridCount + 1` price levels from `lowerPrice` to `upperPrice`,
 * spaced by an equal amount or an equal ratio. The upper level is exact.
 */
export function gridLevels(
	lowerPrice: Decimal,
	upperPrice: Decimal,
	gridCount: number,
	gridType: GridType,
): Decimal[] {
	const step = upperPrice.minus(lowerPrice).div(gridCount);
	const ratio =
		gridType === "geometric"
			? geometricStepRatio(lowerPrice, upperPrice, gridCount)
			: null;
	const levels = [lowerPrice];
	for (let index = 1; index <= gridCount; index += 1) {
		const level =
			index === gridCount
				? upperPrice
				: ratio
					? lowerPrice.times(ratio.pow(index))
					: lowerPrice.plus(step.times(index));
		if (!level.gt(levels[index - 1])) {
			throw new RangeError(
				"Price range is too narrow at the supported precision",
			);
		}
		levels.push(level);
	}
	return levels;
}

/**
 * Returns the largest grid count whose smallest step between neighbouring
 * levels, as a fraction of the lower level, stays at or above `minimumStep`.
 * The result is a whole Decimal, possibly zero or not finite.
 */
export function gridCountForMinimumStep(
	lowerPrice: Decimal,
	upperPrice: Decimal,
	minimumStep: Decimal,
	gridType: GridType,
): Decimal {
	const one = new GridDecimal(1);
	return (
		gridType === "arithmetic"
			? upperPrice
					.minus(lowerPrice)
					.times(one.plus(minimumStep))
					.div(minimumStep.times(upperPrice))
			: upperPrice.div(lowerPrice).ln().div(one.plus(minimumStep).ln())
	).floor();
}

/**
 * Returns the items with the smallest and largest value, the first one on a
 * tie.
 */
export function extremes<Item>(
	items: readonly Item[],
	value: (item: Item) => Decimal,
): { maximum: Item; minimum: Item } {
	if (items.length === 0) throw new RangeError("A grid needs a level");
	let minimum = items[0];
	let maximum = items[0];
	for (const item of items.slice(1)) {
		if (value(item).lt(value(minimum))) minimum = item;
		if (value(item).gt(value(maximum))) maximum = item;
	}
	return { maximum, minimum };
}
