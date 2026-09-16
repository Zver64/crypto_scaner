import type { PriceCandle } from "@/api/candle-history";
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
	cleanSegmentPercent: number;
	cleanShareOfGross: string;
	feeCost: string;
	feeSegmentPercent: number;
	feeShareOfGross: string;
	grossProfit: string;
	isLoss: boolean;
	label: string;
}

export const DEFAULT_MARKUP_PERCENT = 5;
export const DEFAULT_GRID_COUNT = "40";
export const DEFAULT_INVESTMENT = "1000";

function formatCalculatorInput(value: string): string {
	return formatNumber(value).replaceAll(",", "");
}

export interface SpotGridRecommendation {
	input: SpotGridInput;
	hasHourlyVolatility: boolean;
	hasLatestHigh: boolean;
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

export function recommendedLowerPrice(
	upperPrice: string,
	rangePercent: number | undefined,
	gridCount: string,
	gridType: SpotGridType = "geometric",
): string | null {
	if (!validPositiveNumber(rangePercent)) return null;
	try {
		const upper = parseSpotGridDecimal(upperPrice, "Upper price");
		const count = parseSpotGridCount(gridCount);
		const target = new SpotGridDecimal(rangePercent).div(100);
		const lower =
			gridType === "arithmetic"
				? upper
						.times(
							new SpotGridDecimal(1).minus(
								target.times(new SpotGridDecimal(count).minus(1)),
							),
						)
						.div(new SpotGridDecimal(1).plus(target))
				: upper.div(new SpotGridDecimal(1).plus(target).pow(count));
		if (!lower.gt(0)) return null;
		const rounded = lower.toSignificantDigits(3, SpotGridDecimal.ROUND_DOWN);
		const formatted = formatCalculatorInput(rounded.toString());
		const parsed = parseSpotGridDecimal(formatted, "Lower price");
		return parsed.lt(upper) ? formatted : null;
	} catch {
		return null;
	}
}

export function spotGridRecommendation(
	candles: readonly (PriceCandle | null)[] | undefined,
	hourlyVolatilityPercent: number | undefined,
	markupPercent = DEFAULT_MARKUP_PERCENT,
	gridCount = DEFAULT_GRID_COUNT,
): SpotGridRecommendation {
	const high = latestAvailableCandle(candles)?.high;
	const upperPrice = recommendedUpperPrice(high, markupPercent) ?? "";
	const lowerPrice =
		recommendedLowerPrice(upperPrice, hourlyVolatilityPercent, gridCount) ?? "";
	return {
		input: {
			lowerPrice,
			upperPrice,
			gridCount,
			investment: DEFAULT_INVESTMENT,
		},
		hasHourlyVolatility: validPositiveNumber(hourlyVolatilityPercent),
		hasLatestHigh: validPositiveNumber(high),
	};
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
): SpotGridProfitSplit {
	const grossProfit = allocationPerBuy.times(grossProfitPercent).div(100);
	const feeCost = grossProfit.minus(netProfit);
	const feeShareOfGross = feeCost.div(grossProfit).times(100);
	const cleanShareOfGross = netProfit.div(grossProfit).times(100);
	const feeSegmentPercent = feeShareOfGross.gte(100)
		? 100
		: feeShareOfGross.lte(0)
			? 0
			: feeShareOfGross.toNumber();

	return {
		cleanProfit: `${formatNumber(netProfit.toFixed())} USDT`,
		cleanSegmentPercent: 100 - feeSegmentPercent,
		cleanShareOfGross: `${formatNumber(cleanShareOfGross.toFixed())}%`,
		feeCost: `${formatNumber(feeCost.toFixed())} USDT`,
		feeSegmentPercent,
		feeShareOfGross: `${formatNumber(feeShareOfGross.toFixed())}%`,
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
			),
		];
	}

	return [
		profitSplit(
			"Lowest-profit trade",
			estimate.allocationPerBuy,
			estimate.stepPercentMinimum,
			estimate.cycleProfitMinimum,
		),
		profitSplit(
			"Highest-profit trade",
			estimate.allocationPerBuy,
			estimate.stepPercentMaximum,
			estimate.cycleProfitMaximum,
		),
	];
}

export function spotGridEstimateValues(estimate: SpotGridEstimate | null) {
	if (!estimate) {
		return {
			averageEntryPrice: "0 USDT",
			gridStepPercent: "0%",
			profitPerStep: "0 USDT",
			profitPerStepPercent: "0%",
			profitSplits: [
				{
					cleanProfit: "0 USDT",
					cleanSegmentPercent: 0,
					cleanShareOfGross: "0%",
					feeCost: "0 USDT",
					feeSegmentPercent: 0,
					feeShareOfGross: "0%",
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
		profitPerStep: isGeometric
			? `${formatNumber(estimate.cycleProfit.toFixed())} USDT`
			: `${formatNumber(estimate.cycleProfitMinimum.toFixed())}–${formatNumber(estimate.cycleProfitMaximum.toFixed())} USDT`,
		profitPerStepPercent: isGeometric
			? `${formatNumber(estimate.cycleProfitPercent.toFixed())}%`
			: `${formatNumber(estimate.cycleProfitMinimumPercent.toFixed())}%–${formatNumber(estimate.cycleProfitMaximumPercent.toFixed())}%`,
		profitSplits: spotGridProfitSplits(estimate),
	};
}
