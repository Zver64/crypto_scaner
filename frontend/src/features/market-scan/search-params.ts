import {
	type MarketScanCriteria,
	validateMarketScanCriteria,
} from "./criteria";

export interface MarketScanSearch {
	minimum_range_percent?: number;
	percentile?: number;
	period?: number;
	unit?: "days" | "hours";
}

export function parseMarketScanSearch(
	search: Record<string, unknown>,
): MarketScanSearch {
	const period = search.period;
	const unit = search.unit;
	const percentile = search.percentile;
	const minimumRangePercent = search.minimum_range_percent;

	if (
		typeof period !== "number" ||
		(unit !== "days" && unit !== "hours") ||
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
				period,
				unit,
			}),
		).length > 0
	) {
		return {};
	}

	return {
		minimum_range_percent: minimumRangePercent,
		percentile,
		period,
		unit,
	};
}

export function marketScanCriteriaToSearch(
	criteria: MarketScanCriteria,
): Required<MarketScanSearch> {
	return {
		minimum_range_percent: criteria.minimumRangePercent,
		percentile: criteria.percentile,
		period: criteria.period,
		unit: criteria.unit,
	};
}

export function marketScanCriteriaFromSearch(
	search: MarketScanSearch,
): MarketScanCriteria | undefined {
	if (
		search.minimum_range_percent === undefined ||
		search.percentile === undefined ||
		search.period === undefined ||
		search.unit === undefined
	) {
		return undefined;
	}

	return {
		minimumRangePercent: search.minimum_range_percent,
		percentile: search.percentile,
		period: search.period,
		unit: search.unit,
	};
}
