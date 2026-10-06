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
	TRADE_COMPARISON_DIGITS,
	tickRounding,
	UPPER_MARKUP_LIMIT_PERCENT,
	UPPER_MARKUP_MAX_PERCENT,
} from "@/features/instrument-analysis/grid-estimator/config";
import type {
	FuturesGridOptions,
	GridBounds,
	GridCalculation,
	GridEstimateValues,
	GridMarket,
	GridMarketEstimate,
	GridMarkups,
	GridProfitSplit,
	GridRangeCount,
	GridRangeValues,
	GridRecommendation,
	GridTradeAmounts,
	LiquidationRangeBarLayout,
	PriceRounding,
	ProfitSplitColors,
	ProfitSplitRow,
	SpotGridCalculation,
	SpotGridEstimate,
	SpotGridLimits,
} from "@/features/instrument-analysis/grid-estimator/types";
import { calculateArithmeticSpotGrid } from "@/utils/calculator/arithmetic-spot-grid";
import {
	calculateFuturesGrid,
	type FuturesContract,
	type FuturesGridEstimate,
} from "@/utils/calculator/futures-grid";
import { calculateGeometricSpotGrid } from "@/utils/calculator/geometric-spot-grid";
import {
	GRID_MAX_COUNT,
	GridDecimal,
	gridCountForMinimumStep,
	parseGridDecimal,
} from "@/utils/calculator/grid";
import type { GridInput, GridType } from "@/utils/calculator/types";
import { formatNumber } from "@/utils/number-format";
import { formatRangePercent } from "@/utils/range-percent";

// Rounds a grid price to the symbol's Binance tick size, so Binance accepts
// it, or to the shared number format when the tick size is unknown.
function formatCalculatorInput(
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
function formatInputNumber(value: Decimal, rounding?: PriceRounding): string {
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
		const lower = parseGridDecimal(lowerPrice, "Lower price");
		return percentNumber(new GridDecimal(1).minus(lower.div(anchor)));
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
	const { lower, upper } = range;
	try {
		const target = new GridDecimal(stepPercent).div(100);
		let count = gridCountForMinimumStep(lower, upper, target, gridType);
		if (
			validPositiveNumber(anchor) &&
			validPositiveNumber(lowerMarkupPercent) &&
			lowerMarkupPercent < 100
		) {
			const markupLower = new GridDecimal(anchor).times(
				new GridDecimal(1).minus(new GridDecimal(lowerMarkupPercent).div(100)),
			);
			if (upper.gt(markupLower))
				count = GridDecimal.min(
					count,
					gridCountForMinimumStep(markupLower, upper, target, gridType),
				);
		}
		if (!count.isFinite() || count.lt(1)) return null;
		return GridDecimal.min(count, GRID_MAX_COUNT).toString();
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
 * Starts both prices at the given markups, within the range, and the lower
 * price as low as the range allows without a lower markup; the grid count is
 * the largest that keeps the hourly step.
 */
export function gridRecommendation(
	bounds: GridBounds,
	hourlyVolatilityPercent: number | undefined,
	gridType: GridType = "geometric",
	markups: GridMarkups = DEFAULT_MARKUPS.spot,
	investment = DEFAULT_INVESTMENT,
): GridRecommendation {
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
	return formatInputNumber(new GridDecimal(usdt).div(anchor));
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
	input: GridInput,
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
	input: GridInput,
	gridType: GridType = "arithmetic",
): SpotGridCalculation | null {
	return calculateGrid(input, () =>
		gridType === "geometric"
			? calculateGeometricSpotGrid(input)
			: calculateArithmeticSpotGrid(input),
	);
}

export function calculateFuturesGridInput(
	input: GridInput,
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
	unit: string,
	{ grossProfit, profit, profitPercent }: GridTradeAmounts,
	label?: string,
): GridProfitSplit {
	const feeCost = grossProfit.minus(profit);
	const feeShareOfGross = feeCost.div(grossProfit).times(100);
	const feeSegmentPercent = feeShareOfGross.gte(100)
		? 100
		: feeShareOfGross.lte(0)
			? 0
			: feeShareOfGross.toNumber();

	return {
		cleanProfit: formatAmount(profit, unit),
		cleanReturnPercent: `${formatNumber(profitPercent.toFixed())}% per trade`,
		cleanSegmentPercent: 100 - feeSegmentPercent,
		feeCost: formatAmount(feeCost, unit),
		feeSegmentPercent,
		feeShareOfGross: `${formatNumber(feeShareOfGross.toFixed())}% of gross`,
		isLoss: profit.lt(0),
		label,
	};
}

function sameValue(left: Decimal, right: Decimal): boolean {
	return left
		.toSignificantDigits(TRADE_COMPARISON_DIGITS)
		.eq(right.toSignificantDigits(TRADE_COMPARISON_DIGITS));
}

// Splits the lowest- and highest-profit trades, or one trade when both earn
// the same.
function tradeProfitSplits(
	unit: string,
	lowest: GridTradeAmounts,
	highest: GridTradeAmounts,
): GridProfitSplit[] {
	const isSame =
		sameValue(lowest.grossProfit, highest.grossProfit) &&
		sameValue(lowest.profit, highest.profit) &&
		sameValue(lowest.profitPercent, highest.profitPercent);
	return isSame
		? [profitSplit(unit, lowest)]
		: [
				profitSplit(unit, lowest, "Lowest-profit trade"),
				profitSplit(unit, highest, "Highest-profit trade"),
			];
}

export function spotGridProfitSplits(
	estimate: SpotGridEstimate,
): GridProfitSplit[] {
	const trade = (
		stepPercent: Decimal,
		profit: Decimal,
		profitPercent: Decimal,
	): GridTradeAmounts => ({
		grossProfit: estimate.allocationPerBuy.times(stepPercent).div(100),
		profit,
		profitPercent,
	});
	if ("cycleProfit" in estimate) {
		return [
			profitSplit(
				"USDT",
				trade(
					estimate.stepPercent,
					estimate.cycleProfit,
					estimate.cycleProfitPercent,
				),
			),
		];
	}
	return tradeProfitSplits(
		"USDT",
		trade(
			estimate.stepPercentMinimum,
			estimate.cycleProfitMinimum,
			estimate.cycleProfitMinimumPercent,
		),
		trade(
			estimate.stepPercentMaximum,
			estimate.cycleProfitMaximum,
			estimate.cycleProfitMaximumPercent,
		),
	);
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
				isLoss: false,
			},
		],
	};
}

/**
 * Formats the lowest- and highest-profit trades in `unit`, the margin
 * currency, or one trade when both earn the same.
 */
export function futuresGridEstimateValues(
	estimate: FuturesGridEstimate | null,
	unit: string,
): GridEstimateValues {
	if (!estimate) return emptyEstimateValues(unit);
	return {
		profitSplits: tradeProfitSplits(
			unit,
			estimate.tradeMinimum,
			estimate.tradeMaximum,
		),
	};
}

/** Returns the rows of the profit-per-trade bars. */
export function profitSplitRows(
	splits: readonly GridProfitSplit[],
	colors: ProfitSplitColors,
): ProfitSplitRow[] {
	return splits.map((split) => ({
		ariaLabel: `${split.label ?? "Every trade"}: fees ${split.feeCost}, ${split.feeShareOfGross} profit; ${split.isLoss ? "net loss" : "clean profit"} ${split.cleanProfit}, ${split.cleanReturnPercent}`,
		items: [
			{
				color: "orange",
				label: "Fees",
				secondaryValue: split.feeShareOfGross,
				value: split.feeCost,
			},
			{
				color: split.isLoss ? "red" : "green",
				label: split.isLoss ? "Net loss" : "Profit",
				secondaryValue: split.cleanReturnPercent,
				value: split.cleanProfit,
			},
		],
		key: split.label ?? "every-trade",
		label: split.label,
		segments: [
			{ color: colors.fee, key: "fees", percentage: split.feeSegmentPercent },
			{
				color: colors.profit,
				key: "clean-profit",
				percentage: split.cleanSegmentPercent,
			},
		],
	}));
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
			lowerPrice.minus(liquidationPrice).div(lowerPrice).times(100).toFixed(),
			2,
		)}% below the lower price`;
	else if (liquidationPrice?.gt(upperPrice))
		summary = `Liquidation ${formatNumber(
			liquidationPrice.minus(upperPrice).div(upperPrice).times(100).toFixed(),
			2,
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
	input: GridInput,
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
