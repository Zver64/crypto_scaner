import { criterionKeys } from "@/api/analysis-identifiers";
import type { CriterionSelection } from "@/api/client";
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
}

export type MarketScanDraft = {
	[Field in keyof MarketScanCriteria]: number | string;
};

export const defaultMarketScanCriteria: MarketScanCriteria = {
	period: 30,
	percentile: 80,
	minimumRangePercent: 10,
	hourlyPeriod: 60,
	hourlyPercentile: 80,
	hourlyMinimumRangePercent: 2,
	minimumMarketCapMillions: singleVolatilityDefaults.minimumMarketCapMillions,
};

export function marketScanCriteriaIdentity(criteria: MarketScanCriteria) {
	return [
		criteria.period,
		criteria.hourlyPeriod,
		criteria.hourlyPercentile,
		criteria.hourlyMinimumRangePercent,
		criteria.percentile,
		criteria.minimumRangePercent,
		criteria.minimumMarketCapMillions,
	] as const;
}

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
	return errors;
}

export function criterionSelections(
	criteria: MarketScanCriteria,
): CriterionSelection[] {
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
	];
}
