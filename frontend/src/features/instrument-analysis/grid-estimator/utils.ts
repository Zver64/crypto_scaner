import type Decimal from "decimal.js";
import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import {
	BINANCE_GRID_FALLBACK_DEVIATION,
	BINANCE_GRID_FILTER_SHARE,
	DEFAULT_GRID_COUNT,
	DEFAULT_INVESTMENT,
	DEFAULT_MARKUPS,
	FUTURES_CONTRACTS,
	LOWER_MARKUP_LIMIT_PERCENT,
	LOWER_MARKUP_MAX_PERCENT,
	RANGE_BAR_PADDING,
	tickRounding,
	UPPER_MARKUP_LIMIT_PERCENT,
	UPPER_MARKUP_MAX_PERCENT,
} from "@/features/instrument-analysis/grid-estimator/config";
import type {
	FuturesGridOptions,
	GridCalculation,
	GridEstimateValues,
	GridMarket,
	GridMarketEstimate,
	GridMarkups,
	LiquidationRangeBarLayout,
	PriceRounding,
	SpotGridBounds,
	SpotGridCalculation,
	SpotGridEstimate,
	SpotGridLimits,
	SpotGridProfitSplit,
	SpotGridRecommendation,
} from "@/features/instrument-analysis/grid-estimator/types";
import { calculateArithmeticSpotGrid } from "@/utils/calculator/arithmetic-spot-grid";
import {
	calculateFuturesGrid,
	type FuturesContract,
	type FuturesGridEstimate,
} from "@/utils/calculator/futures-grid";
import { calculateGeometricSpotGrid } from "@/utils/calculator/geometric-spot-grid";
import {
	type GridType,
	parseSpotGridDecimal,
	SPOT_GRID_MAX_COUNT,
	SpotGridDecimal,
	type SpotGridInput,
} from "@/utils/calculator/spot-grid";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

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
	return formatInputNumber(value, rounding);
}

// Formats a number as calculator input: the shared number format without
// group separators.
function formatInputNumber(value: Decimal, rounding?: PriceRounding): string {
	return formatNumber(value.toFixed(), undefined, rounding).replaceAll(",", "");
}

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
function formatMinPrice(bounds: SpotGridBounds): string | null {
	return (
		bounds.minPrice && formatCalculatorInput(bounds, bounds.minPrice, "ceil")
	);
}

/** Returns the highest upper price Binance accepts, rounded down for display. */
function formatMaxPrice(bounds: SpotGridBounds): string | null {
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
	priceUnit: string,
): string | null {
	if (!bounds.minPrice) return null;
	try {
		return parseSpotGridDecimal(lowerPrice, "Lower price").lt(bounds.minPrice)
			? `Binance minimum is ${formatMinPrice(bounds)} ${priceUnit}`
			: null;
	} catch {
		return null;
	}
}

/** Returns why an upper price is above the Binance maximum, or null. */
export function upperPriceLimitError(
	bounds: SpotGridBounds,
	upperPrice: string,
	priceUnit: string,
): string | null {
	if (!bounds.maxPrice) return null;
	try {
		return parseSpotGridDecimal(upperPrice, "Upper price").gt(bounds.maxPrice)
			? `Binance maximum is ${formatMaxPrice(bounds)} ${priceUnit}`
			: null;
	} catch {
		return null;
	}
}

function floorGridCount(
	upper: Decimal,
	lower: Decimal,
	target: Decimal,
	gridType: GridType,
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
	gridType: GridType,
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
 * Starts both prices at the given markups, within the range, and the lower
 * price as low as the range allows without a lower markup; the grid count is
 * the largest that keeps the hourly step.
 */
export function gridRecommendation(
	bounds: SpotGridBounds,
	hourlyVolatilityPercent: number | undefined,
	gridType: GridType = "geometric",
	markups: GridMarkups = DEFAULT_MARKUPS.spot,
	investment = DEFAULT_INVESTMENT,
): SpotGridRecommendation {
	const upperMarkup = Math.min(markups.upper, bounds.upperMarkupMax);
	const upperPrice = upperPriceFromMarkup(bounds, upperMarkup) ?? "";
	const lowerMarkup = Math.min(
		markups.lower ?? bounds.lowerMarkupMax,
		bounds.lowerMarkupMax,
	);
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
			investment,
		},
		hasAnchor: bounds.anchor !== null,
		hasHourlyVolatility: validPositiveNumber(hourlyVolatilityPercent),
		lowerMarkup,
		upperMarkup,
	};
}

/**
 * Returns the starting investment of a market: the default USDT amount, or for
 * COIN-M its value in the coin at the anchor price, or one coin without it.
 */
export function defaultInvestment(
	market: GridMarket,
	anchor: number | null,
): string {
	return investmentFromUsdt(market, anchor, DEFAULT_INVESTMENT) ?? "1";
}

/**
 * Converts a USDT amount into the market's margin asset at the anchor price,
 * or returns null when an inverse market has no price to convert at.
 */
export function investmentFromUsdt(
	market: GridMarket,
	anchor: number | null,
	usdt: string,
): string | null {
	if (!isInverseMarket(market)) return usdt;
	if (!validPositiveNumber(anchor)) return null;
	return formatInputNumber(new SpotGridDecimal(usdt).div(anchor));
}

function isInverseMarket(market: GridMarket): boolean {
	return market !== "spot" && FUTURES_CONTRACTS[market] === "inverse";
}

/** Returns the currency a market's grid is margined in. */
export function marginAsset(market: GridMarket, baseAsset: string): string {
	return isInverseMarket(market) ? baseAsset : "USDT";
}

/** Returns the currency a market's prices are quoted in. */
export function priceAsset(market: GridMarket): string {
	return isInverseMarket(market) ? "USD" : "USDT";
}

/** Returns the smallest per-trade step of an estimate, in percent. */
export function spotGridMinimumStepPercent(estimate: SpotGridEstimate): number {
	return (
		"stepPercent" in estimate
			? estimate.stepPercent
			: estimate.stepPercentMinimum
	).toNumber();
}

// Runs a grid calculation once every input field is filled, reporting a
// rejected input as an error rather than throwing.
function calculateGrid<Estimate>(
	input: SpotGridInput,
	calculate: () => Estimate,
): GridCalculation<Estimate> | null {
	if (!Object.values(input).every((value) => value.length > 0)) return null;
	try {
		return { estimate: calculate(), error: null };
	} catch (error) {
		return {
			estimate: null,
			error:
				error instanceof Error ? error.message : "Invalid calculator input",
		};
	}
}

export function calculateSpotGridInput(
	input: SpotGridInput,
	gridType: GridType = "arithmetic",
): SpotGridCalculation | null {
	return calculateGrid(input, () =>
		gridType === "geometric"
			? calculateGeometricSpotGrid(input)
			: calculateArithmeticSpotGrid(input),
	);
}

export function calculateFuturesGridInput(
	input: SpotGridInput,
	{ currentPrice, ...options }: FuturesGridOptions,
	contract: FuturesContract = "linear",
): GridCalculation<FuturesGridEstimate> | null {
	return calculateGrid(input, () => {
		if (currentPrice === null)
			throw new RangeError("The current price is unavailable");
		return calculateFuturesGrid({
			...input,
			...options,
			contract,
			currentPrice,
		});
	});
}

function profitSplit(
	label: string,
	unit: string,
	grossProfit: Decimal,
	netProfit: Decimal,
	netProfitPercent: Decimal,
): SpotGridProfitSplit {
	const feeCost = grossProfit.minus(netProfit);
	const feeShareOfGross = feeCost.div(grossProfit).times(100);
	const feeSegmentPercent = feeShareOfGross.gte(100)
		? 100
		: feeShareOfGross.lte(0)
			? 0
			: feeShareOfGross.toNumber();

	return {
		cleanProfit: formatAmount(netProfit, unit),
		cleanReturnPercent: `${formatNumber(netProfitPercent.toFixed())}% per trade`,
		cleanSegmentPercent: 100 - feeSegmentPercent,
		feeCost: formatAmount(feeCost, unit),
		feeSegmentPercent,
		feeShareOfGross: `${formatNumber(feeShareOfGross.toFixed())}% of gross`,
		grossProfit: formatAmount(grossProfit, unit),
		isLoss: netProfit.lt(0),
		label,
	};
}

export function spotGridProfitSplits(
	estimate: SpotGridEstimate,
): SpotGridProfitSplit[] {
	const split = (
		label: string,
		stepPercent: Decimal,
		netProfit: Decimal,
		netProfitPercent: Decimal,
	) =>
		profitSplit(
			label,
			"USDT",
			estimate.allocationPerBuy.times(stepPercent).div(100),
			netProfit,
			netProfitPercent,
		);
	if ("cycleProfit" in estimate) {
		return [
			split(
				"Every trade",
				estimate.stepPercent,
				estimate.cycleProfit,
				estimate.cycleProfitPercent,
			),
		];
	}

	return [
		split(
			"Lowest-profit trade",
			estimate.stepPercentMinimum,
			estimate.cycleProfitMinimum,
			estimate.cycleProfitMinimumPercent,
		),
		split(
			"Highest-profit trade",
			estimate.stepPercentMaximum,
			estimate.cycleProfitMaximum,
			estimate.cycleProfitMaximumPercent,
		),
	];
}

export function formatAmount(value: Decimal | number, unit: string): string {
	return `${formatNumber(typeof value === "number" ? value : value.toFixed())} ${unit}`;
}

export function spotGridEstimateValues(
	estimate: SpotGridEstimate | null,
): GridEstimateValues {
	if (!estimate) return emptyEstimateValues("USDT");
	return { profitSplits: spotGridProfitSplits(estimate) };
}

function emptyEstimateValues(unit: string): GridEstimateValues {
	const zero = formatAmount(0, unit);
	return {
		profitSplits: [
			{
				cleanProfit: zero,
				cleanReturnPercent: "0% per trade",
				cleanSegmentPercent: 0,
				feeCost: zero,
				feeSegmentPercent: 0,
				feeShareOfGross: "0% of gross",
				grossProfit: zero,
				isLoss: false,
				label: "Every trade",
			},
		],
	};
}

/**
 * Formats the lowest- and highest-profit trades in `unit`, the margin
 * currency, or one trade when both show the same amounts.
 */
export function futuresGridEstimateValues(
	estimate: FuturesGridEstimate | null,
	unit: string,
): GridEstimateValues {
	if (!estimate) return emptyEstimateValues(unit);
	const split = (label: string, trade: FuturesGridEstimate["tradeMinimum"]) =>
		profitSplit(
			label,
			unit,
			trade.grossProfit,
			trade.profit,
			trade.profitPercent,
		);
	const lowest = split("Every trade", estimate.tradeMinimum);
	const highest = split("Every trade", estimate.tradeMaximum);
	const shown = ({
		cleanProfit,
		cleanReturnPercent,
		feeCost,
		feeShareOfGross,
	}: SpotGridProfitSplit) =>
		[cleanProfit, cleanReturnPercent, feeCost, feeShareOfGross].join();
	return {
		profitSplits:
			shown(lowest) === shown(highest)
				? [lowest]
				: [
						{ ...lowest, label: "Lowest-profit trade" },
						{ ...highest, label: "Highest-profit trade" },
					],
	};
}

/**
 * Places the grid range, the current price, and the liquidation price on one
 * horizontal scale that spans all three, and describes how far the
 * liquidation price sits from the grid.
 */
export function liquidationRangeBar(
	lowerPrice: Decimal,
	upperPrice: Decimal,
	currentPrice: number | null,
	liquidationPrice: Decimal | null,
): LiquidationRangeBarLayout {
	const lower = lowerPrice.toNumber();
	const upper = upperPrice.toNumber();
	const liquidation = liquidationPrice?.toNumber() ?? null;
	const prices = [lower, upper, currentPrice, liquidation].filter(
		(price): price is number => price !== null && Number.isFinite(price),
	);
	const minimum = Math.min(...prices);
	const span = Math.max(...prices) - minimum;
	const padding = span * RANGE_BAR_PADDING;
	const position = (price: number) =>
		span > 0 ? ((price - minimum + padding) / (span + 2 * padding)) * 100 : 50;

	const isInsideGrid =
		liquidationPrice?.gte(lowerPrice) === true &&
		liquidationPrice.lte(upperPrice);
	let summary = "No liquidation";
	if (liquidationPrice?.lt(lowerPrice))
		summary = `Liquidation ${formatNumber(
			lowerPrice.minus(liquidationPrice).div(lowerPrice).times(100).toFixed(2),
		)}% below the lower price`;
	else if (liquidationPrice?.gt(upperPrice))
		summary = `Liquidation ${formatNumber(
			liquidationPrice.minus(upperPrice).div(upperPrice).times(100).toFixed(2),
		)}% above the upper price`;
	else if (isInsideGrid) summary = "Liquidation inside the grid range";

	return {
		currentPosition:
			currentPrice !== null && Number.isFinite(currentPrice)
				? position(currentPrice)
				: null,
		gridEndPosition: position(upper),
		gridStartPosition: position(lower),
		isInsideGrid,
		liquidationPosition: liquidation === null ? null : position(liquidation),
		summary,
	};
}

/**
 * Calculates the estimate of the selected market's grid; COIN-M amounts are in
 * `baseAsset`.
 */
export function gridMarketEstimate(
	market: GridMarket,
	input: SpotGridInput,
	options: FuturesGridOptions,
	baseAsset: string,
): GridMarketEstimate {
	if (market === "spot") {
		const calculation = calculateSpotGridInput(input, options.gridType);
		return {
			error: calculation?.error ?? null,
			futuresEstimate: null,
			values: spotGridEstimateValues(calculation?.estimate ?? null),
		};
	}
	const calculation = calculateFuturesGridInput(
		input,
		options,
		FUTURES_CONTRACTS[market],
	);
	const futuresEstimate = calculation?.estimate ?? null;
	return {
		error: calculation?.error ?? null,
		futuresEstimate,
		values: futuresGridEstimateValues(
			futuresEstimate,
			marginAsset(market, baseAsset),
		),
	};
}

/**
 * Returns the scale labels of a markup slider: 0%, the default markup when it
 * lies inside the slider, and the maximum.
 */
export function markupScaleLabels(
	maxPercent: number,
	defaultPercent?: number,
): { label: string; position: number }[] {
	return [
		{ label: "0%", position: 0 },
		...(defaultPercent !== undefined && maxPercent > defaultPercent
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
