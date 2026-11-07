import type Decimal from "decimal.js";

import {
	FUTURES_CONTRACTS,
	RANGE_BAR_PADDING,
	TRADE_COMPARISON_DIGITS,
} from "@/features/instrument-analysis/grid-estimator/config";
import type {
	FuturesGridOptions,
	GridCalculation,
	GridEstimateValues,
	GridMarket,
	GridMarketEstimate,
	GridProfitSplit,
	GridTradeAmounts,
	LiquidationRangeBarLayout,
	ProfitSplitColors,
	ProfitSplitRow,
	SpotGridCalculation,
	SpotGridEstimate,
} from "@/features/instrument-analysis/grid-estimator/types";
import { calculateArithmeticSpotGrid } from "@/utils/calculator/arithmetic-spot-grid";
import {
	calculateFuturesGrid,
	type FuturesContract,
	type FuturesGridEstimate,
} from "@/utils/calculator/futures-grid";
import { calculateGeometricSpotGrid } from "@/utils/calculator/geometric-spot-grid";
import {
	gridLevels,
	parseGridCount,
	parseGridDecimal,
} from "@/utils/calculator/grid";
import type { GridInput, GridType } from "@/utils/calculator/types";
import { formatNumber } from "@/utils/number-format";

export function isInverseMarket(market: GridMarket): boolean {
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

// The ascending price levels of a grid, or null while its range is invalid or
// too narrow to keep its levels apart as numbers. Spot and futures grids share
// their levels.
export function gridPriceLevels(
	{ gridCount, lowerPrice, upperPrice }: GridInput,
	gridType: GridType,
): number[] | null {
	let levels: number[];
	try {
		levels = gridLevels(
			parseGridDecimal(lowerPrice, "Lower price"),
			parseGridDecimal(upperPrice, "Upper price"),
			parseGridCount(gridCount),
			gridType,
		).map((level) => level.toNumber());
	} catch {
		return null;
	}
	return levels.every(
		(level, index) => index === 0 || level > levels[index - 1],
	)
		? levels
		: null;
}
