import type { CriterionRequest } from "@/api/generated/models";
import { applicationConfig } from "@/config";
import { criterionKeys, criterionNames } from "@/features/analysis/identifiers";
import {
	type MarketScanCriteria as SingleVolatilityCriteria,
	defaultMarketScanCriteria as singleVolatilityDefaults,
	criterionSelections as singleVolatilitySelections,
	validateMarketScanCriteria as validateSingleVolatility,
	volatilityCriterionSelection,
} from "@/features/market-scan/criteria";

export interface MarketScanCriteria
	extends Omit<SingleVolatilityCriteria, "unit"> {
	hourlyPeriod: number;
	hourlyPercentile: number;
	hourlyMinimumRangePercent: number;
	dailyMaxRsi: number;
	weeklyMaxRsi: number;
}

export type MarketScanDraft = {
	[Field in keyof MarketScanCriteria]: number | string;
};

// The maximum disables the RSI filter, so its criterion is not requested.
export const rsiFilterConstraints = { minimum: 20, maximum: 100 } as const;

export const defaultMarketScanCriteria: MarketScanCriteria = {
	period: 30,
	percentile: 80,
	minimumRangePercent: applicationConfig.volatility.days.defaultCandleRange,
	hourlyPeriod: 60,
	hourlyPercentile: 80,
	hourlyMinimumRangePercent:
		applicationConfig.volatility.hours.defaultCandleRange,
	minimumMarketCapMillions: singleVolatilityDefaults.minimumMarketCapMillions,
	dailyMaxRsi: rsiFilterConstraints.maximum,
	weeklyMaxRsi: rsiFilterConstraints.maximum,
};

export function validateMarketScanCriteria(values: MarketScanDraft) {
	const errors: Partial<Record<keyof MarketScanCriteria, string>> =
		validateSingleVolatility({ ...values, unit: "days" });
	const hourlyErrors = validateSingleVolatility({
		...values,
		unit: "hours",
		period: values.hourlyPeriod,
		percentile: values.hourlyPercentile,
		minimumRangePercent: values.hourlyMinimumRangePercent,
	});
	if (hourlyErrors.period) errors.hourlyPeriod = hourlyErrors.period;
	if (hourlyErrors.percentile)
		errors.hourlyPercentile = hourlyErrors.percentile;
	if (hourlyErrors.minimumRangePercent)
		errors.hourlyMinimumRangePercent = hourlyErrors.minimumRangePercent;
	validateMaxRsi(values.dailyMaxRsi, "Daily max RSI", "dailyMaxRsi", errors);
	validateMaxRsi(values.weeklyMaxRsi, "Weekly max RSI", "weeklyMaxRsi", errors);
	return errors;
}

function validateMaxRsi(
	value: number | string,
	label: string,
	field: "dailyMaxRsi" | "weeklyMaxRsi",
	errors: Partial<Record<keyof MarketScanCriteria, string>>,
) {
	const { minimum, maximum } = rsiFilterConstraints;
	if (
		typeof value !== "number" ||
		!Number.isFinite(value) ||
		value < minimum ||
		value > maximum
	) {
		errors[field] = `${label} must be between ${minimum} and ${maximum}`;
	}
}

function maxRsiCriterionSelection(
	key: string,
	label: string,
	interval: "1d" | "1w",
	maximum: number,
): CriterionRequest[] {
	return maximum < rsiFilterConstraints.maximum
		? [
				{
					key,
					label,
					name: criterionNames.rsi,
					parameters: { interval, max_rsi: maximum },
				},
			]
		: [];
}

export function criterionSelections(
	criteria: MarketScanCriteria,
): CriterionRequest[] {
	const [daily, ...existingCriteria] = singleVolatilitySelections({
		...criteria,
		unit: "days",
	});
	const hourly = volatilityCriterionSelection({
		...criteria,
		unit: "hours",
		period: criteria.hourlyPeriod,
		percentile: criteria.hourlyPercentile,
		minimumRangePercent: criteria.hourlyMinimumRangePercent,
	});
	return [
		{ ...daily, key: criterionKeys.dailyVolatility, label: "Daily Volatility" },
		{
			...hourly,
			key: criterionKeys.hourlyVolatility,
			label: "Hourly Volatility",
		},
		...existingCriteria,
		...maxRsiCriterionSelection(
			criterionKeys.dailyRsi,
			"Daily RSI",
			"1d",
			criteria.dailyMaxRsi,
		),
		...maxRsiCriterionSelection(
			criterionKeys.weeklyRsi,
			"Weekly RSI",
			"1w",
			criteria.weeklyMaxRsi,
		),
	];
}
