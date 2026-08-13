import {
	type MarketScanCriteria,
	validateMarketScanCriteria,
} from "./criteria";

export interface MarketScanSearch {
	minimum_range_percent?: number;
	percentile?: number;
	period_days?: number;
}

export function parseMarketScanSearch(
	search: Record<string, unknown>,
): MarketScanSearch {
	const periodDays = search.period_days;
	const percentile = search.percentile;
	const minimumRangePercent = search.minimum_range_percent;

	if (
		typeof periodDays !== "number" ||
		typeof percentile !== "number" ||
		typeof minimumRangePercent !== "number"
	) {
		return {};
	}

	if (
		Object.keys(
			validateMarketScanCriteria({
				minimumRangePercent,
				percentile,
				periodDays,
			}),
		).length > 0
	) {
		return {};
	}

	return {
		minimum_range_percent: minimumRangePercent,
		percentile,
		period_days: periodDays,
	};
}

export function marketScanCriteriaToSearch(
	criteria: MarketScanCriteria,
): Required<MarketScanSearch> {
	return {
		minimum_range_percent: criteria.minimumRangePercent,
		percentile: criteria.percentile,
		period_days: criteria.periodDays,
	};
}

export function marketScanCriteriaFromSearch(
	search: MarketScanSearch,
): MarketScanCriteria | undefined {
	if (
		search.minimum_range_percent === undefined ||
		search.percentile === undefined ||
		search.period_days === undefined
	) {
		return undefined;
	}

	return {
		minimumRangePercent: search.minimum_range_percent,
		percentile: search.percentile,
		periodDays: search.period_days,
	};
}
