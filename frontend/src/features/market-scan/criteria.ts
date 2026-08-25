import {
	type AnalysisCriteria,
	type AnalysisDraft,
	defaultAnalysisCriteria,
	validateAnalysisCriteria,
} from "../analysis/criteria";

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
