import {
	type MarketScanCriteria,
	type MarketScanDraft,
	validateMarketScanCriteria,
} from "@/features/market-scan/pipeline";

export function criteriaFromValidDraft(
	values: MarketScanDraft,
): MarketScanCriteria | undefined {
	if (
		Object.keys(validateMarketScanCriteria(values)).length > 0 ||
		typeof values.period !== "number" ||
		typeof values.percentile !== "number" ||
		typeof values.minimumMarketCapMillions !== "number" ||
		typeof values.minimumRangePercent !== "number" ||
		typeof values.hourlyPeriod !== "number" ||
		typeof values.hourlyPercentile !== "number" ||
		typeof values.hourlyMinimumRangePercent !== "number"
	) {
		return undefined;
	}

	return {
		hourlyMinimumRangePercent: values.hourlyMinimumRangePercent,
		hourlyPercentile: values.hourlyPercentile,
		hourlyPeriod: values.hourlyPeriod,
		minimumMarketCapMillions: values.minimumMarketCapMillions,
		minimumRangePercent: values.minimumRangePercent,
		percentile: values.percentile,
		period: values.period,
	};
}
