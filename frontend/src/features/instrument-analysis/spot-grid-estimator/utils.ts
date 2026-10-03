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
export const UPPER_MARKUP_MAX_PERCENT = 50;
export const LOWER_MARKUP_MAX_PERCENT = 50;
// Slider caps when the Binance limits allow more: the lower price stays above
// zero, and the upper price goes no further than triple the average price.
const LOWER_MARKUP_LIMIT_PERCENT = 99;
const UPPER_MARKUP_LIMIT_PERCENT = 200;
export const DEFAULT_GRID_COUNT = "40";
export const DEFAULT_INVESTMENT = "1000";

type PriceRounding = "ceil" | "floor" | undefined;

const tickRounding = {
	ceil: SpotGridDecimal.ROUND_CEIL,
	floor: SpotGridDecimal.ROUND_FLOOR,
	halfExpand: SpotGridDecimal.ROUND_HALF_UP,
};

// Rounds a grid price to the symbol's Binance tick size, so Binance accepts
// it, or to the shared number format when the tick size is unknown.
function formatCalculatorInput(
	bounds: SpotGridBounds,
	value: Decimal,
	rounding?: PriceRounding,
): string {
	if (bounds.tickSize)
		return value
			.toNearest(bounds.tickSize, tickRounding[rounding ?? "halfExpand"])
			.toFixed();
	return formatNumber(value.toFixed(), undefined, rounding).replaceAll(",", "");
}

/** Binance spot grid limits of a symbol, from the grid-limits endpoint. */
export interface SpotGridLimits {
	askMultiplierUp: number;
	averagePrice: number;
	bidMultiplierDown: number;
	maxPrice: number;
	minPrice: number;
	tickSize: number;
}

// Binance grid bots keep their orders inside this share of the deviation the
// exchange's percent price filter allows.
const BINANCE_GRID_FILTER_SHARE = 0.85;
// How far from the average price the range may go when a symbol has no
// percent price filter.
const BINANCE_GRID_FALLBACK_DEVIATION = 0.5;

// Returns the share of the average price the filter's multiplier allows the
// bot to reach, or the fallback deviation when the symbol has no filter.
function gridPriceShare(multiplier: number, direction: 1 | -1): Decimal {
	const deviation =
		multiplier > 0
			? new SpotGridDecimal(multiplier)
					.minus(1)
					.abs()
					.times(BINANCE_GRID_FILTER_SHARE)
			: new SpotGridDecimal(BINANCE_GRID_FALLBACK_DEVIATION);
	return new SpotGridDecimal(1).plus(deviation.times(direction));
}

/**
 * Returns the lowest lower price a Binance spot grid bot accepts, as the bot
 * computes it: max(minPrice, (1 − (1 − bidMultiplierDown) × 0.85) × average),
 * or 50% below the average without a multiplier.
 */
export function binanceGridMinLowerPrice(limits: SpotGridLimits): Decimal {
	return SpotGridDecimal.max(
		Math.max(limits.minPrice, 0),
		new SpotGridDecimal(limits.averagePrice).times(
			gridPriceShare(limits.bidMultiplierDown, -1),
		),
	);
}

/**
 * Returns the highest upper price a Binance spot grid bot accepts, as the bot
 * computes it: min(maxPrice, (1 + (askMultiplierUp − 1) × 0.85) × average),
 * or 50% above the average without a multiplier; a maxPrice of 0 sets no
 * maximum.
 */
export function binanceGridMaxUpperPrice(limits: SpotGridLimits): Decimal {
	const upper = new SpotGridDecimal(limits.averagePrice).times(
		gridPriceShare(limits.askMultiplierUp, 1),
	);
	return limits.maxPrice > 0
		? SpotGridDecimal.min(limits.maxPrice, upper)
		: upper;
}

/**
 * The price both markups are measured from and the price range a Binance grid
 * bot accepts. Without the Binance limits the anchor is the latest hourly
 * close and the prices are not limited.
 */
export interface SpotGridBounds {
	anchor: number | null;
	lowerMarkupMax: number;
	maxPrice: Decimal | null;
	minPrice: Decimal | null;
	tickSize: Decimal | null;
	upperMarkupMax: number;
}

export interface SpotGridRecommendation {
	input: SpotGridInput;
	hasAnchor: boolean;
	hasHourlyVolatility: boolean;
	lowerMarkup: number;
	upperMarkup: number;
}

function validPositiveNumber(
	value: number | null | undefined,
): value is number {
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

// Markups keep two decimals and round toward the anchor, so a slider at its
// maximum stays inside the Binance range.
function markupMax(fraction: Decimal, cap: number): number {
	return Math.max(
		0,
		Math.min(
			cap,
			fraction
				.times(100)
				.toDecimalPlaces(2, SpotGridDecimal.ROUND_DOWN)
				.toNumber(),
		),
	);
}

export function spotGridBounds(
	candles: readonly (PriceCandle | null)[] | undefined,
	limits: SpotGridLimits | null | undefined,
): SpotGridBounds {
	if (!limits || !validPositiveNumber(limits.averagePrice)) {
		const close = latestAvailableCandle(candles)?.close;
		return {
			anchor: validPositiveNumber(close) ? close : null,
			lowerMarkupMax: LOWER_MARKUP_MAX_PERCENT,
			maxPrice: null,
			minPrice: null,
			tickSize: null,
			upperMarkupMax: UPPER_MARKUP_MAX_PERCENT,
		};
	}
	const minPrice = binanceGridMinLowerPrice(limits);
	const maxPrice = binanceGridMaxUpperPrice(limits);
	return {
		anchor: limits.averagePrice,
		lowerMarkupMax: markupMax(
			new SpotGridDecimal(1).minus(minPrice.div(limits.averagePrice)),
			LOWER_MARKUP_LIMIT_PERCENT,
		),
		maxPrice,
		minPrice,
		tickSize: validPositiveNumber(limits.tickSize)
			? new SpotGridDecimal(limits.tickSize)
			: null,
		upperMarkupMax: markupMax(
			maxPrice.div(limits.averagePrice).minus(1),
			UPPER_MARKUP_LIMIT_PERCENT,
		),
	};
}

/** Returns the lowest lower price Binance accepts, rounded up for display. */
export function formatMinPrice(bounds: SpotGridBounds): string | null {
	return (
		bounds.minPrice && formatCalculatorInput(bounds, bounds.minPrice, "ceil")
	);
}

/** Returns the highest upper price Binance accepts, rounded down for display. */
export function formatMaxPrice(bounds: SpotGridBounds): string | null {
	return (
		bounds.maxPrice && formatCalculatorInput(bounds, bounds.maxPrice, "floor")
	);
}

/** Returns the upper price that sits `markupPercent` above the anchor. */
export function upperPriceFromMarkup(
	bounds: SpotGridBounds,
	markupPercent: number,
): string | null {
	const { anchor } = bounds;
	if (
		!validPositiveNumber(anchor) ||
		!Number.isFinite(markupPercent) ||
		markupPercent < 0
	)
		return null;
	try {
		const upper = new SpotGridDecimal(anchor).times(
			new SpotGridDecimal(1).plus(new SpotGridDecimal(markupPercent).div(100)),
		);
		if (!upper.isFinite() || !upper.gt(0)) return null;
		// Never above the Binance maximum.
		const formatted = formatCalculatorInput(bounds, upper);
		return bounds.maxPrice && new SpotGridDecimal(formatted).gt(bounds.maxPrice)
			? formatMaxPrice(bounds)
			: formatted;
	} catch {
		return null;
	}
}

/** Returns the lower price that sits `markupPercent` below the anchor. */
export function lowerPriceFromMarkup(
	bounds: SpotGridBounds,
	markupPercent: number,
): string | null {
	const { anchor } = bounds;
	if (
		!validPositiveNumber(anchor) ||
		!Number.isFinite(markupPercent) ||
		markupPercent < 0 ||
		markupPercent >= 100
	)
		return null;
	try {
		const lower = new SpotGridDecimal(anchor).times(
			new SpotGridDecimal(1).minus(new SpotGridDecimal(markupPercent).div(100)),
		);
		if (!lower.gt(0)) return null;
		// Round down so the range only widens and grid steps never shrink, but
		// never below the Binance minimum.
		const formatted = formatCalculatorInput(bounds, lower, "floor");
		if (bounds.minPrice && new SpotGridDecimal(formatted).lt(bounds.minPrice))
			return formatMinPrice(bounds);
		return parseSpotGridDecimal(formatted, "Lower price").gt(0)
			? formatted
			: null;
	} catch {
		return null;
	}
}

function percentNumber(value: Decimal): number | null {
	const percent = value.times(100).toDecimalPlaces(2).toNumber();
	return Number.isFinite(percent) ? percent : null;
}

/** Returns how far the lower price sits below the anchor, in percent. */
export function lowerMarkupPercent(
	anchor: number | null,
	lowerPrice: string,
): number | null {
	if (!validPositiveNumber(anchor)) return null;
	try {
		const lower = parseSpotGridDecimal(lowerPrice, "Lower price");
		return percentNumber(new SpotGridDecimal(1).minus(lower.div(anchor)));
	} catch {
		return null;
	}
}

/** Returns how far the upper price sits above the anchor, in percent. */
export function upperMarkupPercent(
	anchor: number | null,
	upperPrice: string,
): number | null {
	if (!validPositiveNumber(anchor)) return null;
	try {
		const upper = parseSpotGridDecimal(upperPrice, "Upper price");
		return percentNumber(upper.div(anchor).minus(1));
	} catch {
		return null;
	}
}

/** Returns why a lower price is below the Binance minimum, or null. */
export function lowerPriceLimitError(
	bounds: SpotGridBounds,
	lowerPrice: string,
): string | null {
	if (!bounds.minPrice) return null;
	try {
		return parseSpotGridDecimal(lowerPrice, "Lower price").lt(bounds.minPrice)
			? `Binance minimum is ${formatMinPrice(bounds)} USDT`
			: null;
	} catch {
		return null;
	}
}

/** Returns why an upper price is above the Binance maximum, or null. */
export function upperPriceLimitError(
	bounds: SpotGridBounds,
	upperPrice: string,
): string | null {
	if (!bounds.maxPrice) return null;
	try {
		return parseSpotGridDecimal(upperPrice, "Upper price").gt(bounds.maxPrice)
			? `Binance maximum is ${formatMaxPrice(bounds)} USDT`
			: null;
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
 * from `lowerMarkupPercent` below `anchor`, the count is also capped by the
 * exact markup so rounding the lower price down never adds a grid.
 */
export function gridCountForStep(
	upperPrice: string,
	lowerPrice: string,
	stepPercent: number | undefined,
	gridType: SpotGridType,
	anchor?: number | null,
	lowerMarkupPercent?: number,
): string | null {
	if (!validPositiveNumber(stepPercent)) return null;
	try {
		const upper = parseSpotGridDecimal(upperPrice, "Upper price");
		const lower = parseSpotGridDecimal(lowerPrice, "Lower price");
		if (!upper.gt(lower)) return null;
		const target = new SpotGridDecimal(stepPercent).div(100);
		let count = floorGridCount(upper, lower, target, gridType);
		if (
			validPositiveNumber(anchor) &&
			validPositiveNumber(lowerMarkupPercent) &&
			lowerMarkupPercent < 100
		) {
			const markupLower = new SpotGridDecimal(anchor).times(
				new SpotGridDecimal(1).minus(
					new SpotGridDecimal(lowerMarkupPercent).div(100),
				),
			);
			if (upper.gt(markupLower))
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
 * Starts the upper price at the default markup and the lower price as low as
 * the range allows; the grid count is the largest that keeps the hourly step.
 */
export function spotGridRecommendation(
	bounds: SpotGridBounds,
	hourlyVolatilityPercent: number | undefined,
	gridType: SpotGridType = "geometric",
	markupPercent = DEFAULT_MARKUP_PERCENT,
): SpotGridRecommendation {
	const upperMarkup = Math.min(markupPercent, bounds.upperMarkupMax);
	const upperPrice = upperPriceFromMarkup(bounds, upperMarkup) ?? "";
	const lowerMarkup = bounds.lowerMarkupMax;
	const lowerPrice = lowerPriceFromMarkup(bounds, lowerMarkup) ?? "";
	const gridCount =
		gridCountForStep(
			upperPrice,
			lowerPrice,
			hourlyVolatilityPercent,
			gridType,
			bounds.anchor,
			lowerMarkup,
		) ?? DEFAULT_GRID_COUNT;
	return {
		input: {
			lowerPrice,
			upperPrice,
			gridCount,
			investment: DEFAULT_INVESTMENT,
		},
		hasAnchor: bounds.anchor !== null,
		hasHourlyVolatility: validPositiveNumber(hourlyVolatilityPercent),
		lowerMarkup,
		upperMarkup,
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
