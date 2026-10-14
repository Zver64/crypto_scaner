import type { CriterionRequest, Evaluation } from "@/api/generated/models";
import {
	type AnalysisCriteria,
	defaultAnalysisCriteria,
} from "@/features/analysis/criteria";
import {
	criterionKeys,
	criterionNames,
	evaluationMetricKeys,
} from "@/features/analysis/identifiers";
import { usdPerMillion } from "@/utils/market-cap";

export interface MarketScanCriteria extends AnalysisCriteria {
	minimumMarketCapMillions: number;
	minimumRangePercent: number;
}

export const defaultMarketScanCriteria: MarketScanCriteria = {
	...defaultAnalysisCriteria,
	minimumMarketCapMillions: 500,
	minimumRangePercent: 3,
};

export const marketScanCriteriaConstraints = {
	minimumMarketCapMillions: { minimum: 0 },
	minimumRangePercent: { minimum: 0 },
} as const;

export function volatilityCriterionSelection(
	criteria: MarketScanCriteria,
): CriterionRequest {
	return {
		key: criterionKeys.volatility,
		label: "Volatility",
		name: criterionNames.volatility,
		parameters: {
			minimum_range_percent: criteria.minimumRangePercent,
			percentile: criteria.percentile,
			period: criteria.period,
			unit: criteria.unit,
		},
	};
}

export function criterionSelections(
	criteria: MarketScanCriteria,
): CriterionRequest[] {
	return [
		volatilityCriterionSelection(criteria),
		{
			key: criterionKeys.marketCap,
			label: "Market Cap",
			name: criterionNames.marketCap,
			parameters: {
				min_market_cap_usd: criteria.minimumMarketCapMillions * usdPerMillion,
			},
		},
	];
}

export interface VolatilityEvaluation {
	candleCount: number;
	from: string;
	matched: boolean;
	rangePercent: number;
	to: string;
}

export function volatilityEvaluation(
	evaluations: readonly Evaluation[],
	instanceKey: string = criterionKeys.volatility,
): VolatilityEvaluation | undefined {
	const evaluation = evaluations.find(({ key }) => key === instanceKey);
	const rangePercent = evaluation?.metrics[evaluationMetricKeys.rangePercent];
	if (!evaluation || typeof rangePercent !== "number") {
		return undefined;
	}

	return {
		candleCount: evaluation.candle_count,
		from: evaluation.from,
		matched: evaluation.matched,
		rangePercent,
		to: evaluation.to,
	};
}
