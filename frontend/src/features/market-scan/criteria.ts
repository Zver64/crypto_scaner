import type { CriterionSelection, Evaluation } from "../../api/client";
import {
	type AnalysisCriteria,
	type AnalysisDraft,
	defaultAnalysisCriteria,
	validateAnalysisCriteria,
} from "../analysis/criteria";

export const volatilityCriterionKey = "volatility";
export const volatilityCriterionName = "volatility";
export const marketCapCriterionName = "market_cap";
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
		key: volatilityCriterionKey,
		label: "Volatility",
		name: volatilityCriterionName,
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
	const selections = [volatilityCriterionSelection(criteria)];
	if (criteria.minimumMarketCapMillions > 0) {
		selections.push({
			key: marketCapCriterionName,
			label: "Market Cap",
			name: marketCapCriterionName,
			parameters: {
				min_market_cap_usd: criteria.minimumMarketCapMillions * usdPerMillion,
			},
		});
	}
	return selections;
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
	instanceKey = volatilityCriterionKey,
): VolatilityEvaluation | undefined {
	const evaluation = evaluations.find(({ key }) => key === instanceKey);
	const rangePercent = evaluation?.metrics.range_percent;
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

export interface MarketCapEvaluation {
	marketCapUsd: number;
	matched: boolean;
}

export function marketCapEvaluation(
	evaluations: readonly Evaluation[],
): MarketCapEvaluation | undefined {
	const evaluation = evaluations.find(
		({ key }) => key === marketCapCriterionName,
	);
	const marketCapUsd = evaluation?.metrics.market_cap_usd;
	if (!evaluation || typeof marketCapUsd !== "number") {
		return undefined;
	}

	return { marketCapUsd, matched: evaluation.matched };
}
