export type AnalysisUnit = "days" | "hours";

export interface AnalysisCriteria {
	percentile: number;
	period: number;
	unit: AnalysisUnit;
}

export interface AnalysisDraft {
	percentile: number | string;
	period: number | string;
	unit: AnalysisUnit;
}

export const defaultAnalysisCriteria: AnalysisCriteria = {
	percentile: 80,
	period: 30,
	unit: "days",
};

export const analysisCriteriaConstraints = {
	percentile: { maximum: 100, minimum: 0 },
	period: { maximum: 87600, minimum: 1 },
} as const;

export function defaultPeriodForUnit(unit: AnalysisUnit): number {
	return unit === "hours" ? 60 : 30;
}

export function maximumPeriodForUnit(unit: AnalysisUnit): number {
	return unit === "hours" ? 87600 : 3650;
}

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
		analysisCriteriaConstraints.period.minimum,
		maximumPeriodForUnit(values.unit),
		`Analysis period must be a whole number between 1 and ${maximumPeriodForUnit(values.unit)} ${values.unit}`,
		(message) => {
			errors.period = message;
		},
	);
	validateInteger(
		values.percentile,
		"Range percentile",
		analysisCriteriaConstraints.percentile.minimum,
		analysisCriteriaConstraints.percentile.maximum,
		"Range percentile must be between 0 and 100",
		(message) => {
			errors.percentile = message;
		},
	);

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
