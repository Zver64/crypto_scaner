import {
	type MarketScanCriteria,
	type MarketScanDraft,
	marketScanCriteriaIdentity,
} from "@/features/market-scan/pipeline";

export function criteriaAreEqual(
	left: MarketScanCriteria,
	right: MarketScanCriteria,
): boolean {
	const leftIdentity = marketScanCriteriaIdentity(left);
	const rightIdentity = marketScanCriteriaIdentity(right);
	return leftIdentity.every((value, index) => value === rightIdentity[index]);
}

export function criteriaFromValidDraft(
	values: MarketScanDraft,
): MarketScanCriteria | undefined {
	if (
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
