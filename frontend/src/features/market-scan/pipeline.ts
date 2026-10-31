import type { z } from "zod";

import type { CriterionRequest } from "@/api/generated/models";
import { applicationConfig } from "@/config";
import { defaultVolatilitySettings } from "@/features/analysis/criteria";
import { criterionKeys } from "@/features/analysis/identifiers";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import {
	criterionSelections as singleVolatilitySelections,
	volatilityCriterionSelection,
} from "@/features/market-scan/criteria";
import type { marketScanCriteriaSchema } from "@/features/market-scan/schemas";

export type MarketScanCriteria = z.output<typeof marketScanCriteriaSchema>;

// The criteria as the number inputs emit them.
export type MarketScanDraft = z.input<typeof marketScanCriteriaSchema>;

export const defaultMarketScanCriteria: MarketScanCriteria = {
	...defaultVolatilitySettings,
	minimumRangePercent: applicationConfig.volatility.days.defaultCandleRange,
	hourlyMinimumRangePercent:
		applicationConfig.volatility.hours.defaultCandleRange,
	minimumMarketCapMillions: applicationConfig.marketCap.defaultMinimumMillions,
};

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
	];
}

// The criteria of settings without filters: every coin matches, so pages
// that only read the volatility, such as Top Market Cap, Favorites, and the
// coin page, get both ranges of every coin.
export function unfilteredScanCriteria(
	settings: VolatilitySettings,
): MarketScanCriteria {
	return {
		...settings,
		hourlyMinimumRangePercent: 0,
		minimumMarketCapMillions: 0,
		minimumRangePercent: 0,
	};
}

// The unfiltered daily and hourly volatility criteria without Market Cap,
// for the coin page, which reads only the volatility.
export function volatilityCriterionSelections(
	settings: VolatilitySettings,
): CriterionRequest[] {
	return criterionSelections(unfilteredScanCriteria(settings)).filter(
		({ key }) => key !== criterionKeys.marketCap,
	);
}
