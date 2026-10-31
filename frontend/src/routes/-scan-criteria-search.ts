import type { MarketScanCriteria } from "@/features/market-scan/pipeline";
import { marketScanCriteriaSchema } from "@/features/market-scan/schemas";

export interface ScanCriteriaSearch {
	hourly_minimum_range_percent?: number;
	hourly_percentile?: number;
	hourly_period?: number;
	minimum_market_cap_millions?: number;
	minimum_range_percent?: number;
	percentile?: number;
	period?: number;
}

export function parseOptionalScanCriteriaSearch(
	search: Record<string, unknown>,
): ScanCriteriaSearch {
	const criteria = scanCriteriaFromSearch(search);
	return criteria ? scanCriteriaToSearch(criteria) : {};
}

export function scanCriteriaToSearch(
	criteria: MarketScanCriteria,
): Required<ScanCriteriaSearch> {
	return {
		hourly_minimum_range_percent: criteria.hourlyMinimumRangePercent,
		hourly_percentile: criteria.hourlyPercentile,
		hourly_period: criteria.hourlyPeriod,
		minimum_market_cap_millions: criteria.minimumMarketCapMillions,
		minimum_range_percent: criteria.minimumRangePercent,
		percentile: criteria.percentile,
		period: criteria.period,
	};
}

export function scanCriteriaFromSearch(
	search: ScanCriteriaSearch | Record<string, unknown>,
): MarketScanCriteria | undefined {
	return marketScanCriteriaSchema.safeParse({
		hourlyMinimumRangePercent: search.hourly_minimum_range_percent,
		hourlyPercentile: search.hourly_percentile,
		hourlyPeriod: search.hourly_period,
		minimumMarketCapMillions:
			search.minimum_market_cap_millions === undefined
				? 0
				: search.minimum_market_cap_millions,
		minimumRangePercent: search.minimum_range_percent,
		percentile: search.percentile,
		period: search.period,
	}).data;
}
