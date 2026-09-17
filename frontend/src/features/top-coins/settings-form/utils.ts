import {
	type AnalysisValidationErrors,
	validateAnalysisCriteria,
} from "@/features/analysis/criteria";
import type {
	TopCoinsSettings,
	TopCoinsSettingsDraft,
	TopCoinsSettingsValidationErrors,
} from "@/features/top-coins/settings-form/types";

export function validateTopCoinsSettings(
	values: TopCoinsSettingsDraft,
): TopCoinsSettingsValidationErrors {
	const errors: TopCoinsSettingsValidationErrors = validateAnalysisCriteria({
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
	values: TopCoinsSettingsDraft,
): TopCoinsSettings | undefined {
	if (
		Object.keys(validateTopCoinsSettings(values)).length > 0 ||
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
