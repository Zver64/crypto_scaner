import {
	defaultMarketScanCriteria,
	type MarketScanCriteria,
	validateMarketScanCriteria,
} from "@/features/market-scan/pipeline";

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

export function parseRequiredScanCriteriaSearch(
	search: Record<string, unknown>,
): Required<ScanCriteriaSearch> {
	return scanCriteriaToSearch(
		scanCriteriaFromSearch(search) ?? defaultMarketScanCriteria,
	);
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
	const period = search.period;
	const hourlyPeriod = search.hourly_period;
	const hourlyPercentile = search.hourly_percentile;
	const hourlyMinimumRangePercent = search.hourly_minimum_range_percent;
	const percentile = search.percentile;
	const minimumRangePercent = search.minimum_range_percent;
	const minimumMarketCapMillions =
		search.minimum_market_cap_millions === undefined
			? 0
			: search.minimum_market_cap_millions;

	if (
		typeof period !== "number" ||
		typeof hourlyPeriod !== "number" ||
		typeof hourlyPercentile !== "number" ||
		typeof hourlyMinimumRangePercent !== "number" ||
		typeof percentile !== "number" ||
		typeof minimumRangePercent !== "number" ||
		typeof minimumMarketCapMillions !== "number"
	) {
		return undefined;
	}

	const criteria = {
		hourlyMinimumRangePercent,
		hourlyPercentile,
		hourlyPeriod,
		minimumMarketCapMillions,
		minimumRangePercent,
		percentile,
		period,
	};
	return Object.keys(validateMarketScanCriteria(criteria)).length === 0
		? criteria
		: undefined;
}

export function requiredScanCriteriaFromSearch(
	search: Required<ScanCriteriaSearch>,
): MarketScanCriteria {
	return scanCriteriaFromSearch(search) ?? defaultMarketScanCriteria;
}
