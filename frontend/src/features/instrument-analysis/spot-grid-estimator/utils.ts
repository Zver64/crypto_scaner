import {
	type ArithmeticSpotGridEstimate,
	type ArithmeticSpotGridInput,
	calculateArithmeticSpotGrid,
} from "@/utils/calculator/arithmetic-spot-grid";
import { formatNumber } from "@/utils/number-format";

export interface SpotGridCalculation {
	error: string | null;
	estimate: ArithmeticSpotGridEstimate | null;
}

export function calculateSpotGridInput(
	input: ArithmeticSpotGridInput,
): SpotGridCalculation | null {
	if (!Object.values(input).every((value) => value.length > 0)) {
		return null;
	}

	try {
		return { estimate: calculateArithmeticSpotGrid(input), error: null };
	} catch (error) {
		return {
			estimate: null,
			error:
				error instanceof Error ? error.message : "Invalid calculator input",
		};
	}
}

export function spotGridEstimateValues(
	estimate: ArithmeticSpotGridEstimate | null,
) {
	return {
		averageEntryPrice: `${estimate ? formatNumber(estimate.averageEntryPrice.toFixed()) : "0"} USDT`,
		profitPerStep: estimate
			? `${formatNumber(estimate.cycleProfitMinimum.toFixed())}–${formatNumber(estimate.cycleProfitMaximum.toFixed())} USDT`
			: "0 USDT",
		profitPerStepPercent: estimate
			? `${formatNumber(estimate.cycleProfitMinimumPercent.toFixed())}%–${formatNumber(estimate.cycleProfitMaximumPercent.toFixed())}%`
			: "0%",
	};
}
