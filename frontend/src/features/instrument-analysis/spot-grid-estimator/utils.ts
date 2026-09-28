import type Decimal from "decimal.js";
import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import {
	type ArithmeticSpotGridEstimate,
	calculateArithmeticSpotGrid,
} from "@/utils/calculator/arithmetic-spot-grid";
import {
	calculateGeometricSpotGrid,
	type GeometricSpotGridEstimate,
} from "@/utils/calculator/geometric-spot-grid";
import {
	parseSpotGridCount,
	parseSpotGridDecimal,
	SPOT_GRID_MAX_COUNT,
	SpotGridDecimal,
	type SpotGridInput,
} from "@/utils/calculator/spot-grid";
import { formatNumber } from "@/utils/number-format";

export type SpotGridType = "arithmetic" | "geometric";
export type SpotGridEstimate =
	| ArithmeticSpotGridEstimate
	| GeometricSpotGridEstimate;

export interface SpotGridCalculation {
	error: string | null;
	estimate: SpotGridEstimate | null;
}

export interface SpotGridProfitSplit {
	cleanProfit: string;
	// Net return of the trade relative to its order, e.g. "0.52% per trade".
	cleanReturnPercent: string;
	cleanSegmentPercent: number;
	feeCost: string;
	feeSegmentPercent: number;
	feeShareOfGross: string;
	grossProfit: string;
	isLoss: boolean;
	label: string;
}

export const DEFAULT_MARKUP_PERCENT = 5;
export const LOWER_MARKUP_MAX_PERCENT = 50;
export const DEFAULT_GRID_COUNT = "40";
export const DEFAULT_INVESTMENT = "1000";

function formatCalculatorInput(value: string): string {
	return formatNumber(value).replaceAll(",", "");
}

export interface SpotGridRecommendation {
	input: SpotGridInput;
	hasHourlyVolatility: boolean;
	hasLatestHigh: boolean;
	lowerMarkup: number | null;
}

function validPositiveNumber(value: number | undefined): value is number {
	return typeof value === "number" && Number.isFinite(value) && value > 0;
}

/** Returns the most recent available candle, including histories with trailing gaps. */
export function latestAvailableCandle(
	candles: readonly (PriceCandle | null)[] | undefined,
): PriceCandle | null {
	if (!candles) return null;
	for (let index = candles.length - 1; index >= 0; index -= 1) {
		if (candles[index]) return candles[index];
	}
	return null;
}

export function recommendedUpperPrice(
	high: number | undefined,
	markupPercent: number,
): string | null {
	if (
		!validPositiveNumber(high) ||
		!Number.isFinite(markupPercent) ||
		markupPercent < 0
	)
		return null;
	try {
		const upper = new SpotGridDecimal(high).times(
			new SpotGridDecimal(1).plus(new SpotGridDecimal(markupPercent).div(100)),
		);
		return upper.isFinite() && upper.gt(0)
			? formatCalculatorInput(upper.toString())
			: null;
	} catch {
		return null;
	}
}

function roundedLowerPrice(lower: Decimal, upper: Decimal): string | null {
	if (!lower.gt(0)) return null;
	// Round down so the range only widens and grid steps never shrink.
	const formatted = formatNumber(
		lower.toFixed(),
		undefined,
		"floor",
	).replaceAll(",", "");
	const parsed = parseSpotGridDecimal(formatted, "Lower price");
	return parsed.lte(upper) ? formatted : null;
}

/** Returns the lower price that sits `markupPercent` below the upper price. */
export function lowerPriceFromMarkup(
	upperPrice: string,
	markupPercent: number,
): string | null {
	if (
		!Number.isFinite(markupPercent) ||
		markupPercent < 0 ||
		markupPercent >= 100
	)
		return null;
	try {
		const upper = parseSpotGridDecimal(upperPrice, "Upper price");
		const lower = upper.times(
			new SpotGridDecimal(1).minus(new SpotGridDecimal(markupPercent).div(100)),
		);
		return roundedLowerPrice(lower, upper);
	} catch {
		return null;
	}
}

function percentNumber(value: Decimal): number | null {
	const percent = value.times(100).toDecimalPlaces(2).toNumber();
	return Number.isFinite(percent) ? percent : null;
}

/** Returns how far the lower price sits below the upper price, in percent. */
export function lowerMarkupPercent(
	upperPrice: string,
	lowerPrice: string,
): number | null {
	try {
		const upper = parseSpotGridDecimal(upperPrice, "Upper price");
		const lower = parseSpotGridDecimal(lowerPrice, "Lower price");
		return percentNumber(new SpotGridDecimal(1).minus(lower.div(upper)));
	} catch {
		return null;
	}
}

/** Returns how far the upper price sits above the candle high, in percent. */
export function upperMarkupPercent(
	high: number | undefined,
	upperPrice: string,
): number | null {
	if (!validPositiveNumber(high)) return null;
	try {
		const upper = parseSpotGridDecimal(upperPrice, "Upper price");
		return percentNumber(upper.div(high).minus(1));
	} catch {
		return null;
	}
}

function floorGridCount(
	upper: Decimal,
	lower: Decimal,
	target: Decimal,
	gridType: SpotGridType,
): Decimal {
	return (
		gridType === "arithmetic"
			? upper
					.minus(lower)
					.times(new SpotGridDecimal(1).plus(target))
					.div(target.times(upper))
			: upper.div(lower).ln().div(new SpotGridDecimal(1).plus(target).ln())
	).floor();
}

/**
 * Returns the largest grid count whose minimum step stays at or above
 * `stepPercent` for the given price range. When the lower price was derived
 * from `lowerMarkupPercent`, the count is also capped by the exact markup so
 * rounding the lower price down never adds a grid.
 */
export function gridCountForStep(
	upperPrice: string,
	lowerPrice: string,
	stepPercent: number | undefined,
	gridType: SpotGridType,
	lowerMarkupPercent?: number,
): string | null {
	if (!validPositiveNumber(stepPercent)) return null;
	try {
		const upper = parseSpotGridDecimal(upperPrice, "Upper price");
		const lower = parseSpotGridDecimal(lowerPrice, "Lower price");
		if (!upper.gt(lower)) return null;
		const target = new SpotGridDecimal(stepPercent).div(100);
		let count = floorGridCount(upper, lower, target, gridType);
		if (validPositiveNumber(lowerMarkupPercent) && lowerMarkupPercent < 100) {
			const markupLower = upper.times(
				new SpotGridDecimal(1).minus(
					new SpotGridDecimal(lowerMarkupPercent).div(100),
				),
			);
			count = SpotGridDecimal.min(
				count,
				floorGridCount(upper, markupLower, target, gridType),
			);
		}
		if (!count.isFinite() || count.lt(1)) return null;
		return SpotGridDecimal.min(count, SPOT_GRID_MAX_COUNT).toString();
	} catch {
		return null;
	}
}

/**
 * Returns the smallest whole lower markup percent that fits `gridCount` grids
 * with a minimum step of `stepPercent`, capped at the slider maximum.
 */
export function initialLowerMarkup(
	stepPercent: number | undefined,
	gridCount: string,
	gridType: SpotGridType,
): number | null {
	if (!validPositiveNumber(stepPercent)) return null;
	try {
		const count = parseSpotGridCount(gridCount);
		const target = new SpotGridDecimal(stepPercent).div(100);
		const markup =
			gridType === "arithmetic"
				? target.times(count).div(new SpotGridDecimal(1).plus(target))
				: new SpotGridDecimal(1).minus(
						new SpotGridDecimal(1).plus(target).pow(-count),
					);
		return Math.min(
			LOWER_MARKUP_MAX_PERCENT,
			markup.times(100).ceil().toNumber(),
		);
	} catch {
		return null;
	}
}

export function spotGridRecommendation(
	candles: readonly (PriceCandle | null)[] | undefined,
	hourlyVolatilityPercent: number | undefined,
	gridType: SpotGridType = "geometric",
	markupPercent = DEFAULT_MARKUP_PERCENT,
	targetGridCount = DEFAULT_GRID_COUNT,
): SpotGridRecommendation {
	const high = latestAvailableCandle(candles)?.high;
	const upperPrice = recommendedUpperPrice(high, markupPercent) ?? "";
	const lowerMarkup = initialLowerMarkup(
		hourlyVolatilityPercent,
		targetGridCount,
		gridType,
	);
	const lowerPrice =
		lowerMarkup === null
			? ""
			: (lowerPriceFromMarkup(upperPrice, lowerMarkup) ?? "");
	const gridCount =
		gridCountForStep(
			upperPrice,
			lowerPrice,
			hourlyVolatilityPercent,
			gridType,
			lowerMarkup ?? undefined,
		) ?? targetGridCount;
	return {
		input: {
			lowerPrice,
			upperPrice,
			gridCount,
			investment: DEFAULT_INVESTMENT,
		},
		hasHourlyVolatility: validPositiveNumber(hourlyVolatilityPercent),
		hasLatestHigh: validPositiveNumber(high),
		lowerMarkup,
	};
}

/** Returns the smallest per-trade step of an estimate, in percent. */
export function spotGridMinimumStepPercent(estimate: SpotGridEstimate): number {
	return (
		"stepPercent" in estimate
			? estimate.stepPercent
			: estimate.stepPercentMinimum
	).toNumber();
}

export function calculateSpotGridInput(
	input: SpotGridInput,
	gridType: SpotGridType = "arithmetic",
): SpotGridCalculation | null {
	if (!Object.values(input).every((value) => value.length > 0)) return null;
	try {
		return {
			estimate:
				gridType === "geometric"
					? calculateGeometricSpotGrid(input)
					: calculateArithmeticSpotGrid(input),
			error: null,
		};
	} catch (error) {
		return {
			estimate: null,
			error:
				error instanceof Error ? error.message : "Invalid calculator input",
		};
	}
}

function profitSplit(
	label: string,
	allocationPerBuy: GeometricSpotGridEstimate["allocationPerBuy"],
	grossProfitPercent: GeometricSpotGridEstimate["stepPercent"],
	netProfit: GeometricSpotGridEstimate["cycleProfit"],
	netProfitPercent: GeometricSpotGridEstimate["cycleProfitPercent"],
): SpotGridProfitSplit {
	const grossProfit = allocationPerBuy.times(grossProfitPercent).div(100);
	const feeCost = grossProfit.minus(netProfit);
	const feeShareOfGross = feeCost.div(grossProfit).times(100);
	const feeSegmentPercent = feeShareOfGross.gte(100)
		? 100
		: feeShareOfGross.lte(0)
			? 0
			: feeShareOfGross.toNumber();

	return {
		cleanProfit: `${formatNumber(netProfit.toFixed())} USDT`,
		cleanReturnPercent: `${formatNumber(netProfitPercent.toFixed())}% per trade`,
		cleanSegmentPercent: 100 - feeSegmentPercent,
		feeCost: `${formatNumber(feeCost.toFixed())} USDT`,
		feeSegmentPercent,
		feeShareOfGross: `${formatNumber(feeShareOfGross.toFixed())}% of gross`,
		grossProfit: `${formatNumber(grossProfit.toFixed())} USDT`,
		isLoss: netProfit.lt(0),
		label,
	};
}

export function spotGridProfitSplits(
	estimate: SpotGridEstimate,
): SpotGridProfitSplit[] {
	if ("cycleProfit" in estimate) {
		return [
			profitSplit(
				"Every trade",
				estimate.allocationPerBuy,
				estimate.stepPercent,
				estimate.cycleProfit,
				estimate.cycleProfitPercent,
			),
		];
	}

	return [
		profitSplit(
			"Lowest-profit trade",
			estimate.allocationPerBuy,
			estimate.stepPercentMinimum,
			estimate.cycleProfitMinimum,
			estimate.cycleProfitMinimumPercent,
		),
		profitSplit(
			"Highest-profit trade",
			estimate.allocationPerBuy,
			estimate.stepPercentMaximum,
			estimate.cycleProfitMaximum,
			estimate.cycleProfitMaximumPercent,
		),
	];
}

export function spotGridEstimateValues(estimate: SpotGridEstimate | null) {
	if (!estimate) {
		return {
			averageEntryPrice: "0 USDT",
			gridStepPercent: "0%",
			profitSplits: [
				{
					cleanProfit: "0 USDT",
					cleanReturnPercent: "0% per trade",
					cleanSegmentPercent: 0,
					feeCost: "0 USDT",
					feeSegmentPercent: 0,
					feeShareOfGross: "0% of gross",
					grossProfit: "0 USDT",
					isLoss: false,
					label: "Every trade",
				},
			],
		};
	}
	const isGeometric = "cycleProfit" in estimate;
	return {
		averageEntryPrice: `${formatNumber(estimate.averageEntryPrice.toFixed())} USDT`,
		gridStepPercent: isGeometric
			? `${formatNumber(estimate.stepPercent.toFixed())}%`
			: `${formatNumber(estimate.stepPercentMinimum.toFixed())}%–${formatNumber(estimate.stepPercentMaximum.toFixed())}%`,
		profitSplits: spotGridProfitSplits(estimate),
	};
}
