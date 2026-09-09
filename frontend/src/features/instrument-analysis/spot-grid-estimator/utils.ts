import {
	type ArithmeticSpotGridEstimate,
	calculateArithmeticSpotGrid,
} from "@/utils/calculator/arithmetic-spot-grid";
import {
	calculateGeometricSpotGrid,
	type GeometricSpotGridEstimate,
} from "@/utils/calculator/geometric-spot-grid";
import type { SpotGridInput } from "@/utils/calculator/spot-grid";
import { formatNumber } from "@/utils/number-format";

export type SpotGridType = "arithmetic" | "geometric";
export type SpotGridEstimate =
	| ArithmeticSpotGridEstimate
	| GeometricSpotGridEstimate;

export interface SpotGridCalculation {
	error: string | null;
	estimate: SpotGridEstimate | null;
}

export function calculateSpotGridInput(
	input: SpotGridInput,
	gridType: SpotGridType = "arithmetic",
): SpotGridCalculation | null {
	if (!Object.values(input).every((value) => value.length > 0)) {
		return null;
	}

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

export function spotGridEstimateValues(estimate: SpotGridEstimate | null) {
	if (!estimate) {
		return {
			averageEntryPrice: "0 USDT",
			profitPerStep: "0 USDT",
			profitPerStepPercent: "0%",
		};
	}

	const isGeometric = "cycleProfit" in estimate;
	return {
		averageEntryPrice: `${formatNumber(estimate.averageEntryPrice.toFixed())} USDT`,
		profitPerStep: isGeometric
			? `${formatNumber(estimate.cycleProfit.toFixed())} USDT`
			: `${formatNumber(estimate.cycleProfitMinimum.toFixed())}–${formatNumber(estimate.cycleProfitMaximum.toFixed())} USDT`,
		profitPerStepPercent: isGeometric
			? `${formatNumber(estimate.cycleProfitPercent.toFixed())}%`
			: `${formatNumber(estimate.cycleProfitMinimumPercent.toFixed())}%–${formatNumber(estimate.cycleProfitMaximumPercent.toFixed())}%`,
	};
}
