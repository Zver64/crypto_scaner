import type Decimal from "decimal.js";

import {
	formatCalculatorInput,
	lowerMarkupPercent,
	upperMarkupPercent,
	validPositiveNumber,
} from "@/features/instrument-analysis/grid-estimator/bounds";
import { DEFAULT_INVESTMENT } from "@/features/instrument-analysis/grid-estimator/config";
import {
	calculateSpotGridInput,
	spotGridMinimumStepPercent,
} from "@/features/instrument-analysis/grid-estimator/estimate";
import type {
	GridBounds,
	GridEstimatorFormValues,
	GridRangeCount,
	GridRangeValues,
} from "@/features/instrument-analysis/grid-estimator/types";
import {
	GRID_MAX_COUNT,
	GridDecimal,
	gridCountForMinimumStep,
	gridRangeRatio,
	parseGridDecimal,
} from "@/utils/calculator/grid";
import type { GridInput, GridType } from "@/utils/calculator/types";
import { formatRangePercent } from "@/utils/range-percent";

// Parses a price range whose upper price lies above its lower price.
function parsedRange(
	lowerPrice: string,
	upperPrice: string,
): { lower: Decimal; upper: Decimal } | null {
	try {
		const upper = parseGridDecimal(upperPrice, "Upper price");
		const lower = parseGridDecimal(lowerPrice, "Lower price");
		return upper.gt(lower) ? { lower, upper } : null;
	} catch {
		return null;
	}
}

// Returns the lower price a grid count is derived from: the exact price
// `lowerMarkupPercent` below `anchor` when the lower price was rounded down
// from it, so the rounding never adds a grid, or else the lower price.
function countedLowerPrice(
	lower: Decimal,
	upper: Decimal,
	anchor: number | null | undefined,
	lowerMarkupPercent: number | undefined,
): Decimal {
	if (
		!validPositiveNumber(anchor) ||
		!validPositiveNumber(lowerMarkupPercent) ||
		lowerMarkupPercent >= 100
	)
		return lower;
	const markupLower = new GridDecimal(anchor).times(
		new GridDecimal(1).minus(new GridDecimal(lowerMarkupPercent).div(100)),
	);
	return upper.gt(markupLower) ? GridDecimal.max(lower, markupLower) : lower;
}

// Returns the uncapped grid count of a range and the lower price it is derived
// from, or null when not even one step fits.
function rangeGridCount(
	lower: Decimal,
	upper: Decimal,
	stepPercent: number,
	gridType: GridType,
	anchor: number | null | undefined,
	lowerMarkupPercent: number | undefined,
): { count: Decimal; countedLower: Decimal } | null {
	const countedLower = countedLowerPrice(
		lower,
		upper,
		anchor,
		lowerMarkupPercent,
	);
	const count = gridCountForMinimumStep(
		countedLower,
		upper,
		new GridDecimal(stepPercent).div(100),
		gridType,
	);
	return count.isFinite() && count.gte(1) ? { count, countedLower } : null;
}

/**
 * Returns the largest grid count whose minimum step stays at or above
 * `stepPercent` for the given price range. When the lower price was derived
 * from `lowerMarkupPercent` below `anchor`, the count is also capped by the
 * exact markup so rounding the lower price down never adds a grid.
 */
export function gridCountForStep(
	upperPrice: string,
	lowerPrice: string,
	stepPercent: number | undefined,
	gridType: GridType,
	anchor?: number | null,
	lowerMarkupPercent?: number,
): string | null {
	if (!validPositiveNumber(stepPercent)) return null;
	const range = parsedRange(lowerPrice, upperPrice);
	if (!range) return null;
	try {
		const derived = rangeGridCount(
			range.lower,
			range.upper,
			stepPercent,
			gridType,
			anchor,
			lowerMarkupPercent,
		);
		return derived && GridDecimal.min(derived.count, GRID_MAX_COUNT).toString();
	} catch {
		return null;
	}
}

// Returns a range with its grid count, the lower price that count is derived
// from, and the ratio of the upper to the lower price at which that many grids
// keep their minimum step exactly at `rangePercent`. Returns null when the
// range has no such grid, or when the grid count is capped, which keeps the
// steps wider than the selected one.
function evenGridRange(
	bounds: GridBounds,
	range: GridRangeValues,
	lowerMarkup: number,
) {
	const { anchor } = bounds;
	const parsed = parsedRange(range.lowerPrice, range.upperPrice);
	if (
		!validPositiveNumber(anchor) ||
		!validPositiveNumber(range.rangePercent) ||
		!parsed
	)
		return null;
	const derived = rangeGridCount(
		parsed.lower,
		parsed.upper,
		range.rangePercent,
		range.gridType,
		anchor,
		lowerMarkup,
	);
	if (!derived || derived.count.gt(GRID_MAX_COUNT)) return null;
	const ratio = gridRangeRatio(
		new GridDecimal(range.rangePercent).div(100),
		derived.count.toNumber(),
		range.gridType,
	);
	return (
		ratio && {
			...parsed,
			anchor: new GridDecimal(anchor),
			countedLower: derived.countedLower,
			gridCount: derived.count.toString(),
			ratio,
		}
	);
}

// Returns the resolution formatCalculatorInput rounds a price to: the tick
// size, or else the last of four significant digits but at least a whole unit.
function priceUnit(bounds: GridBounds, price: Decimal): Decimal {
	return (
		bounds.tickSize ??
		new GridDecimal(10).pow(Math.max(-18, Math.min(0, price.e - 3)))
	);
}

/**
 * Returns the upper price moved down to where the grid's minimum step is
 * exactly `rangePercent` with its grid count kept, and its markup. Returns
 * null when the range is already even up to price rounding, narrower than one
 * step, or capped at the maximum grid count, or when the price would fall
 * below the anchor.
 */
export function evenUpperBound(
	bounds: GridBounds,
	range: GridRangeValues,
	lowerMarkup: number,
): Pick<GridEstimatorFormValues, "markup" | "upperPrice"> | null {
	try {
		const even = evenGridRange(bounds, range, lowerMarkup);
		if (!even) return null;
		// Measure from the lower price the grid count is derived from, and round
		// up so the rounding never takes a grid away.
		const upperPrice = formatCalculatorInput(
			bounds,
			even.countedLower.times(even.ratio),
			"ceil",
		);
		const upper = new GridDecimal(upperPrice);
		const markup = upperMarkupPercent(bounds.anchor, upperPrice);
		// A move within the rounding of both prices leaves the grid as even.
		const rounding = priceUnit(bounds, even.upper).plus(
			priceUnit(bounds, even.lower).times(even.ratio),
		);
		if (
			markup === null ||
			even.upper.minus(upper).lte(rounding) ||
			upper.lt(even.anchor) ||
			gridCountForStep(
				upperPrice,
				range.lowerPrice,
				range.rangePercent,
				range.gridType,
				bounds.anchor,
				lowerMarkup,
			) !== even.gridCount
		)
			return null;
		return { markup, upperPrice };
	} catch {
		return null;
	}
}

/**
 * Returns the lower price moved up to where the grid's minimum step is exactly
 * `rangePercent` with its grid count kept, and its markup. Returns null when
 * the range is already even up to price rounding, narrower than one step, or
 * capped at the maximum grid count, or when the price would rise above the
 * anchor.
 */
export function evenLowerBound(
	bounds: GridBounds,
	range: GridRangeValues,
	lowerMarkup: number,
): Pick<GridEstimatorFormValues, "lowerMarkup" | "lowerPrice"> | null {
	try {
		const even = evenGridRange(bounds, range, lowerMarkup);
		if (!even) return null;
		// Round the price down and its markup up, so neither the rounding nor
		// the markup cap of gridCountForStep takes a grid away.
		const lowerPrice = formatCalculatorInput(
			bounds,
			even.upper.div(even.ratio),
			"floor",
		);
		const lower = new GridDecimal(lowerPrice);
		const markup = lowerMarkupPercent(bounds.anchor, lowerPrice, "ceil");
		// A move within the rounding of both prices leaves the grid as even.
		const rounding = priceUnit(bounds, even.lower).plus(
			priceUnit(bounds, even.upper).div(even.ratio),
		);
		if (
			markup === null ||
			lower.minus(even.lower).lte(rounding) ||
			lower.gt(even.anchor) ||
			gridCountForStep(
				range.upperPrice,
				lowerPrice,
				range.rangePercent,
				range.gridType,
				bounds.anchor,
				markup,
			) !== even.gridCount
		)
			return null;
		return { lowerMarkup: markup, lowerPrice };
	} catch {
		return null;
	}
}

/**
 * Returns the grid count of a new range: the largest that keeps its minimum
 * step at `rangePercent`. Keeps `gridCount`, with an error, when the range is
 * narrower than one step, and without one when the range is not valid yet or
 * no step is selected.
 */
export function gridCountForRange(
	range: GridRangeValues,
	gridCount: string,
	anchor: number | null,
	lowerMarkup: number,
): GridRangeCount {
	if (!(range.rangePercent > 0)) return { error: null, gridCount };
	const derived = gridCountForStep(
		range.upperPrice,
		range.lowerPrice,
		range.rangePercent,
		range.gridType,
		anchor,
		lowerMarkup,
	);
	if (derived !== null) return { error: null, gridCount: derived };
	return {
		error: parsedRange(range.lowerPrice, range.upperPrice)
			? `The price range is narrower than one ${formatRangePercent(range.rangePercent)} step, so the grid count is unchanged`
			: null,
		gridCount,
	};
}

/**
 * Returns the minimum step of a grid count, or null when the prices or the
 * count are not valid. It depends only on the levels, which spot and futures
 * grids share, so any investment gives it.
 */
export function gridCountStepPercent(
	input: Omit<GridInput, "investment">,
	gridType: GridType,
): number | null {
	const estimate = calculateSpotGridInput(
		{ ...input, investment: DEFAULT_INVESTMENT },
		gridType,
	)?.estimate;
	return estimate ? spotGridMinimumStepPercent(estimate) : null;
}

/**
 * Returns why a grid count's minimum step leaves the hourly-to-daily range, or
 * null when it stays inside or no hourly range bounds it.
 */
export function gridCountStepError(
	stepPercent: number | null,
	hourlyRange: number | null,
	dailyRange: number | null,
): string | null {
	if (
		stepPercent === null ||
		hourlyRange === null ||
		(stepPercent >= hourlyRange &&
			(dailyRange === null || stepPercent <= dailyRange))
	) {
		return null;
	}
	return `Minimum grid step would be ${formatRangePercent(stepPercent)}; it must be ${
		dailyRange === null
			? `at least ${formatRangePercent(hourlyRange)}`
			: `from ${formatRangePercent(hourlyRange)} to ${formatRangePercent(dailyRange)}`
	}`;
}
