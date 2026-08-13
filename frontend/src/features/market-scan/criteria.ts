export interface AnalysisCriteria {
	percentile: number;
	periodDays: number;
}

export interface AnalysisDraft {
	percentile: number | string;
	periodDays: number | string;
}

export interface MarketScanCriteria extends AnalysisCriteria {
	minimumRangePercent: number;
}

export interface MarketScanDraft extends AnalysisDraft {
	minimumRangePercent: number | string;
}

export const defaultMarketScanCriteria: MarketScanCriteria = {
	minimumRangePercent: 3,
	percentile: 80,
	periodDays: 30,
};

export const marketScanCriteriaConstraints = {
	minimumRangePercent: { minimum: 0 },
	percentile: { maximum: 100, minimum: 0 },
	periodDays: { maximum: 3650, minimum: 1 },
} as const;

export type MarketScanValidationErrors = Partial<
	Record<keyof MarketScanCriteria, string>
>;

export type AnalysisValidationErrors = Partial<
	Record<keyof AnalysisCriteria, string>
>;

export function validateAnalysisCriteria(
	values: AnalysisDraft,
): AnalysisValidationErrors {
	const errors: AnalysisValidationErrors = {};

	validateInteger(
		values.periodDays,
		"Analysis period",
		marketScanCriteriaConstraints.periodDays.minimum,
		marketScanCriteriaConstraints.periodDays.maximum,
		"Analysis period must be between 1 and 3650 days",
		(message) => {
			errors.periodDays = message;
		},
	);
	validateInteger(
		values.percentile,
		"Range percentile",
		marketScanCriteriaConstraints.percentile.minimum,
		marketScanCriteriaConstraints.percentile.maximum,
		"Range percentile must be between 0 and 100",
		(message) => {
			errors.percentile = message;
		},
	);

	return errors;
}

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

function validateInteger(
	value: number | string,
	label: string,
	minimum: number,
	maximum: number,
	rangeMessage: string,
	setError: (message: string) => void,
) {
	if (value === "") {
		setError(`${label} is required`);
	} else if (typeof value !== "number" || !Number.isFinite(value)) {
		setError(`${label} must be a number`);
	} else if (!Number.isInteger(value)) {
		setError(`${label} must be a whole number`);
	} else if (value < minimum || value > maximum) {
		setError(rangeMessage);
	}
}
