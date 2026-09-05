import {
	criterionKeys,
	criterionNames,
	evaluationMetricKeys,
} from "@/api/analysis-identifiers";
import type { CriterionSelection, Evaluation } from "@/api/client";
import {
	type AnalysisCriteria,
	type AnalysisDraft,
	defaultAnalysisCriteria,
	validateAnalysisCriteria,
} from "@/features/analysis/criteria";

const usdPerMillion = 1_000_000;

export interface MarketScanCriteria extends AnalysisCriteria {
	minimumMarketCapMillions: number;
	minimumRangePercent: number;
}

export interface MarketScanDraft extends AnalysisDraft {
	minimumMarketCapMillions: number | string;
	minimumRangePercent: number | string;
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

export type MarketScanValidationErrors = Partial<
	Record<keyof MarketScanCriteria, string>
>;

export function validateMarketScanCriteria(
	values: MarketScanDraft,
): MarketScanValidationErrors {
	const errors: MarketScanValidationErrors = validateAnalysisCriteria(values);
	validateNonNegativeNumber(
		values.minimumMarketCapMillions,
		"Minimum market cap",
		"minimumMarketCapMillions",
		errors,
	);
	validateNonNegativeNumber(
		values.minimumRangePercent,
		"Minimum range",
		"minimumRangePercent",
		errors,
	);

	return errors;
}

function validateNonNegativeNumber(
	value: number | string,
	label: string,
	field: keyof MarketScanCriteria,
	errors: MarketScanValidationErrors,
) {
	if (value === "") {
		errors[field] = `${label} is required`;
	} else if (typeof value !== "number" || !Number.isFinite(value)) {
		errors[field] = `${label} must be a number`;
	} else if (value < 0) {
		errors[field] = `${label} must be zero or greater`;
	}
}

export function volatilityCriterionSelection(
	criteria: MarketScanCriteria,
): CriterionSelection {
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
): CriterionSelection[] {
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
