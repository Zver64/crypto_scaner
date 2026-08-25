import type { CriterionSelection, Evaluation } from "../../api/client";
import {
	type AnalysisCriteria,
	type AnalysisDraft,
	defaultAnalysisCriteria,
	validateAnalysisCriteria,
} from "../analysis/criteria";

export const percentileCriterionName = "percentile";

export interface MarketScanCriteria extends AnalysisCriteria {
	minimumRangePercent: number;
}

export interface MarketScanDraft extends AnalysisDraft {
	minimumRangePercent: number | string;
}

export const defaultMarketScanCriteria: MarketScanCriteria = {
	...defaultAnalysisCriteria,
	minimumRangePercent: 3,
};

export const marketScanCriteriaConstraints = {
	minimumRangePercent: { minimum: 0 },
} as const;

export type MarketScanValidationErrors = Partial<
	Record<keyof MarketScanCriteria, string>
>;

export function validateMarketScanCriteria(
	values: MarketScanDraft,
): MarketScanValidationErrors {
	const errors: MarketScanValidationErrors = validateAnalysisCriteria(values);

	if (values.minimumRangePercent === "") {
		errors.minimumRangePercent = "Minimum range is required";
	} else if (
		typeof values.minimumRangePercent !== "number" ||
		!Number.isFinite(values.minimumRangePercent)
	) {
		errors.minimumRangePercent = "Minimum range must be a number";
	} else if (
		values.minimumRangePercent <
		marketScanCriteriaConstraints.minimumRangePercent.minimum
	) {
		errors.minimumRangePercent = "Minimum range must be zero or greater";
	}

	return errors;
}

export function percentileCriterionSelection(
	criteria: MarketScanCriteria,
): CriterionSelection {
	return {
		name: percentileCriterionName,
		parameters: {
			minimum_range_percent: criteria.minimumRangePercent,
			percentile: criteria.percentile,
			period: criteria.period,
			unit: criteria.unit,
		},
	};
}

export interface PercentileEvaluation {
	candleCount: number;
	from: string;
	matched: boolean;
	rangePercent: number;
	to: string;
}

export function percentileEvaluation(
	evaluations: readonly Evaluation[],
): PercentileEvaluation | undefined {
	const evaluation = evaluations.find(
		({ name }) => name === percentileCriterionName,
	);
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
