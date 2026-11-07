import type Decimal from "decimal.js";

import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import {
	BINANCE_GRID_FALLBACK_DEVIATION,
	BINANCE_GRID_FILTER_SHARE,
	DEFAULT_MARKUP_PERCENT,
	LOWER_MARKUP_LIMIT_PERCENT,
	LOWER_MARKUP_MAX_PERCENT,
	tickRounding,
	UPPER_MARKUP_LIMIT_PERCENT,
	UPPER_MARKUP_MAX_PERCENT,
} from "@/features/instrument-analysis/grid-estimator/config";
import type {
	GridBounds,
	GridMarket,
	PriceRounding,
	SpotGridLimits,
} from "@/features/instrument-analysis/grid-estimator/types";
import { GridDecimal, parseGridDecimal } from "@/utils/calculator/grid";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

// Rounds a grid price to the symbol's Binance tick size, so Binance accepts
// it, or to the shared number format when the tick size is unknown.
export function formatCalculatorInput(
	bounds: GridBounds,
	value: Decimal,
	rounding?: PriceRounding,
): string {
	if (bounds.tickSize)
		return value
			.toNearest(bounds.tickSize, tickRounding[rounding ?? "halfExpand"])
			.toFixed();
	return formatInputNumber(value, rounding);
}

// Formats a number as calculator input: the shared number format without
// group separators.
export function formatInputNumber(
	value: Decimal,
	rounding?: PriceRounding,
): string {
	return formatNumber(value.toFixed(), undefined, rounding, false);
}

// Returns the share of the average price the filter's multiplier allows the
// bot to reach, or the fallback deviation when the symbol has no filter.
function gridPriceShare(multiplier: number, direction: 1 | -1): Decimal {
	const deviation =
		multiplier > 0
			? new GridDecimal(multiplier)
					.minus(1)
					.abs()
					.times(BINANCE_GRID_FILTER_SHARE)
			: new GridDecimal(BINANCE_GRID_FALLBACK_DEVIATION);
	return new GridDecimal(1).plus(deviation.times(direction));
}

/**
 * Returns the lowest lower price a Binance spot grid bot accepts, as the bot
 * computes it: max(minPrice, (1 − (1 − bidMultiplierDown) × 0.85) × average),
 * or 50% below the average without a multiplier.
 */
function binanceGridMinLowerPrice(limits: SpotGridLimits): Decimal {
	return GridDecimal.max(
		Math.max(limits.minPrice, 0),
		new GridDecimal(limits.averagePrice).times(
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
function binanceGridMaxUpperPrice(limits: SpotGridLimits): Decimal {
	const upper = new GridDecimal(limits.averagePrice).times(
		gridPriceShare(limits.askMultiplierUp, 1),
	);
	return limits.maxPrice > 0 ? GridDecimal.min(limits.maxPrice, upper) : upper;
}

export function validPositiveNumber(
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
			fraction.times(100).toDecimalPlaces(2, GridDecimal.ROUND_DOWN).toNumber(),
		),
	);
}

/**
 * Returns the anchor and the price range of a market's grid. Only spot grids
 * follow the Binance spot grid limits; futures grids anchor at the same price
 * without them.
 */
export function gridBounds(
	market: GridMarket,
	candles: readonly (PriceCandle | null)[] | undefined,
	limits: SpotGridLimits | null | undefined,
): GridBounds {
	const averagePrice = limits?.averagePrice;
	if (market === "spot" && limits && validPositiveNumber(averagePrice))
		return binanceGridBounds(limits, averagePrice);
	const close = latestAvailableCandle(candles)?.close;
	return {
		anchor: validPositiveNumber(averagePrice)
			? averagePrice
			: validPositiveNumber(close)
				? close
				: null,
		lowerMarkupMax: LOWER_MARKUP_MAX_PERCENT,
		maxPrice: null,
		minPrice: null,
		tickSize: null,
		upperMarkupMax: UPPER_MARKUP_MAX_PERCENT,
	};
}

function binanceGridBounds(
	limits: SpotGridLimits,
	averagePrice: number,
): GridBounds {
	const minPrice = binanceGridMinLowerPrice(limits);
	const maxPrice = binanceGridMaxUpperPrice(limits);
	return {
		anchor: averagePrice,
		lowerMarkupMax: markupMax(
			new GridDecimal(1).minus(minPrice.div(averagePrice)),
			LOWER_MARKUP_LIMIT_PERCENT,
		),
		maxPrice,
		minPrice,
		tickSize: validPositiveNumber(limits.tickSize)
			? new GridDecimal(limits.tickSize)
			: null,
		upperMarkupMax: markupMax(
			maxPrice.div(averagePrice).minus(1),
			UPPER_MARKUP_LIMIT_PERCENT,
		),
	};
}

/** Returns the lowest lower price Binance accepts, rounded up for display. */
function formatMinPrice(bounds: GridBounds): string | null {
	return (
		bounds.minPrice && formatCalculatorInput(bounds, bounds.minPrice, "ceil")
	);
}

/** Returns the highest upper price Binance accepts, rounded down for display. */
function formatMaxPrice(bounds: GridBounds): string | null {
	return (
		bounds.maxPrice && formatCalculatorInput(bounds, bounds.maxPrice, "floor")
	);
}

/** Returns the upper price that sits `markupPercent` above the anchor. */
export function upperPriceFromMarkup(
	bounds: GridBounds,
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
		const upper = new GridDecimal(anchor).times(
			new GridDecimal(1).plus(new GridDecimal(markupPercent).div(100)),
		);
		if (!upper.isFinite() || !upper.gt(0)) return null;
		// Never above the Binance maximum.
		const formatted = formatCalculatorInput(bounds, upper);
		return bounds.maxPrice && new GridDecimal(formatted).gt(bounds.maxPrice)
			? formatMaxPrice(bounds)
			: formatted;
	} catch {
		return null;
	}
}

/** Returns the lower price that sits `markupPercent` below the anchor. */
export function lowerPriceFromMarkup(
	bounds: GridBounds,
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
		const lower = new GridDecimal(anchor).times(
			new GridDecimal(1).minus(new GridDecimal(markupPercent).div(100)),
		);
		if (!lower.gt(0)) return null;
		// Round down so the range only widens and grid steps never shrink, but
		// never below the Binance minimum.
		const formatted = formatCalculatorInput(bounds, lower, "floor");
		if (bounds.minPrice && new GridDecimal(formatted).lt(bounds.minPrice))
			return formatMinPrice(bounds);
		return parseGridDecimal(formatted, "Lower price").gt(0) ? formatted : null;
	} catch {
		return null;
	}
}

function percentNumber(
	value: Decimal,
	rounding?: PriceRounding,
): number | null {
	const percent = value
		.times(100)
		.toDecimalPlaces(2, tickRounding[rounding ?? "halfExpand"])
		.toNumber();
	return Number.isFinite(percent) ? percent : null;
}

/** Returns how far the lower price sits below the anchor, in percent. */
export function lowerMarkupPercent(
	anchor: number | null,
	lowerPrice: string,
	rounding?: PriceRounding,
): number | null {
	if (!validPositiveNumber(anchor)) return null;
	try {
		const lower = parseGridDecimal(lowerPrice, "Lower price");
		return percentNumber(new GridDecimal(1).minus(lower.div(anchor)), rounding);
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
		const upper = parseGridDecimal(upperPrice, "Upper price");
		return percentNumber(upper.div(anchor).minus(1));
	} catch {
		return null;
	}
}

/** Returns why a lower price is below the Binance minimum, or null. */
export function lowerPriceLimitError(
	bounds: GridBounds,
	lowerPrice: string,
	priceUnit: string,
): string | null {
	if (!bounds.minPrice) return null;
	try {
		return parseGridDecimal(lowerPrice, "Lower price").lt(bounds.minPrice)
			? `Binance minimum is ${formatMinPrice(bounds)} ${priceUnit}`
			: null;
	} catch {
		return null;
	}
}

/** Returns why an upper price is above the Binance maximum, or null. */
export function upperPriceLimitError(
	bounds: GridBounds,
	upperPrice: string,
	priceUnit: string,
): string | null {
	if (!bounds.maxPrice) return null;
	try {
		return parseGridDecimal(upperPrice, "Upper price").gt(bounds.maxPrice)
			? `Binance maximum is ${formatMaxPrice(bounds)} ${priceUnit}`
			: null;
	} catch {
		return null;
	}
}

/**
 * Returns the scale labels of a markup slider: 0%, the default markup when it
 * lies inside the slider, and the maximum.
 */
export function markupScaleLabels(
	maxPercent: number,
	defaultPercent = DEFAULT_MARKUP_PERCENT,
): { label: string; position: number }[] {
	return [
		{ label: "0%", position: 0 },
		...(maxPercent > defaultPercent
			? [
					{
						label: formatRangePercent(defaultPercent),
						position: (defaultPercent / maxPercent) * 100,
					},
				]
			: []),
		{ label: formatRangePercent(maxPercent), position: 100 },
	];
}
