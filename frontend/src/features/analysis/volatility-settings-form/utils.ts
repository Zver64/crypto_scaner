import {
	type AnalysisValidationErrors,
	validateAnalysisCriteria,
} from "@/features/analysis/criteria";
import type {
	VolatilitySettings,
	VolatilitySettingsDraft,
	VolatilitySettingsValidationErrors,
} from "@/features/analysis/volatility-settings-form/types";

export function validateVolatilitySettings(
	values: VolatilitySettingsDraft,
): VolatilitySettingsValidationErrors {
	const errors: VolatilitySettingsValidationErrors = validateAnalysisCriteria({
		percentile: values.percentile,
		period: values.period,
		unit: "days",
	});
	const hourlyErrors: AnalysisValidationErrors = validateAnalysisCriteria({
		percentile: values.hourlyPercentile,
		period: values.hourlyPeriod,
		unit: "hours",
	});

	if (hourlyErrors.period) errors.hourlyPeriod = hourlyErrors.period;
	if (hourlyErrors.percentile)
		errors.hourlyPercentile = hourlyErrors.percentile;

	return errors;
}

export function settingsFromValidDraft(
	values: VolatilitySettingsDraft,
): VolatilitySettings | undefined {
	if (
		Object.keys(validateVolatilitySettings(values)).length > 0 ||
		typeof values.period !== "number" ||
		typeof values.percentile !== "number" ||
		typeof values.hourlyPeriod !== "number" ||
		typeof values.hourlyPercentile !== "number"
	) {
		return undefined;
	}

	return {
		hourlyPercentile: values.hourlyPercentile,
		hourlyPeriod: values.hourlyPeriod,
		percentile: values.percentile,
		period: values.period,
	};
}
