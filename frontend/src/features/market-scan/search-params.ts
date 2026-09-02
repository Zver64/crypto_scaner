import {
	type MarketScanCriteria,
	validateMarketScanCriteria,
} from "./pipeline";

export interface MarketScanSearch {
	minimum_market_cap_millions?: number;
	minimum_range_percent?: number;
	percentile?: number;
	period?: number;
	hourly_period?: number;
	hourly_percentile?: number;
	hourly_minimum_range_percent?: number;
}

export function parseMarketScanSearch(
	search: Record<string, unknown>,
): MarketScanSearch {
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
		return {};
	}

	if (
		Object.keys(
			validateMarketScanCriteria({
				minimumMarketCapMillions,
				minimumRangePercent,
				percentile,
				period,
				hourlyPeriod,
				hourlyPercentile,
				hourlyMinimumRangePercent,
			}),
		).length > 0
	) {
		return {};
	}

	const parsedSearch: MarketScanSearch = {
		minimum_market_cap_millions: minimumMarketCapMillions,
		minimum_range_percent: minimumRangePercent,
		percentile,
		period,
		hourly_period: hourlyPeriod,
		hourly_percentile: hourlyPercentile,
		hourly_minimum_range_percent: hourlyMinimumRangePercent,
	};
	return parsedSearch;
}

export function marketScanCriteriaToSearch(
	criteria: MarketScanCriteria,
): Required<MarketScanSearch> {
	return {
		minimum_market_cap_millions: criteria.minimumMarketCapMillions,
		minimum_range_percent: criteria.minimumRangePercent,
		percentile: criteria.percentile,
		period: criteria.period,
		hourly_period: criteria.hourlyPeriod,
		hourly_percentile: criteria.hourlyPercentile,
		hourly_minimum_range_percent: criteria.hourlyMinimumRangePercent,
	};
}

export function marketScanCriteriaFromSearch(
	search: MarketScanSearch,
): MarketScanCriteria | undefined {
	if (
		search.minimum_market_cap_millions === undefined ||
		search.minimum_range_percent === undefined ||
		search.percentile === undefined ||
		search.period === undefined ||
		search.hourly_period === undefined ||
		search.hourly_percentile === undefined ||
		search.hourly_minimum_range_percent === undefined
	) {
		return undefined;
	}

	return {
		minimumMarketCapMillions: search.minimum_market_cap_millions,
		minimumRangePercent: search.minimum_range_percent,
		percentile: search.percentile,
		period: search.period,
		hourlyPeriod: search.hourly_period,
		hourlyPercentile: search.hourly_percentile,
		hourlyMinimumRangePercent: search.hourly_minimum_range_percent,
	};
}
