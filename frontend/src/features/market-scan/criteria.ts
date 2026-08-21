export interface AnalysisCriteria {
	percentile: number;
	unit: AnalysisUnit;
	period: number;
}

export interface AnalysisDraft {
	percentile: number | string;
	period: number | string;
	unit: AnalysisUnit;
}

export interface MarketScanCriteria extends AnalysisCriteria {
	minimumRangePercent: number;
}

export interface MarketScanDraft extends AnalysisDraft {
	minimumRangePercent: number | string;
}

export type AnalysisUnit = "days" | "hours";

export const defaultMarketScanCriteria: MarketScanCriteria = {
	minimumRangePercent: 3,
	percentile: 80,
	period: 30,
	unit: "days",
};

export const marketScanCriteriaConstraints = {
	minimumRangePercent: { minimum: 0 },
	percentile: { maximum: 100, minimum: 0 },
	period: { maximum: 87600, minimum: 1 },
} as const;

export function defaultPeriodForUnit(unit: AnalysisUnit): number {
	return unit === "hours" ? 60 : 30;
}

export function maximumPeriodForUnit(unit: AnalysisUnit): number {
	return unit === "hours" ? 87600 : 3650;
}

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
		values.period,
		"Analysis period",
		marketScanCriteriaConstraints.period.minimum,
		maximumPeriodForUnit(values.unit),
		`Analysis period must be a whole number between 1 and ${maximumPeriodForUnit(values.unit)} ${values.unit}`,
		(message) => {
			errors.period = message;
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
