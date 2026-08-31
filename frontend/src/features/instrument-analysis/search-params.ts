import {
	defaultMarketScanCriteria,
	type MarketScanCriteria,
	validateMarketScanCriteria,
} from "../market-scan/pipeline";

export interface InstrumentAnalysisSearch {
	hourly_minimum_range_percent: number;
	hourly_percentile: number;
	hourly_period: number;
	percentile: number;
	period: number;
	minimum_range_percent: number;
	minimum_market_cap_millions: number;
}

export function parseInstrumentAnalysisSearch(
	search: Record<string, unknown> | InstrumentAnalysisSearch,
): InstrumentAnalysisSearch {
	const period = search.period;
	const percentile = search.percentile;
	const minimumRangePercent = search.minimum_range_percent;
	const hourlyPeriod = search.hourly_period;
	const hourlyPercentile = search.hourly_percentile;
	const hourlyMinimumRangePercent = search.hourly_minimum_range_percent;
	const minimumMarketCapMillions =
		search.minimum_market_cap_millions === undefined
			? 0
			: search.minimum_market_cap_millions;

	if (
		typeof period !== "number" ||
		typeof percentile !== "number" ||
		typeof minimumRangePercent !== "number" ||
		typeof hourlyPeriod !== "number" ||
		typeof hourlyPercentile !== "number" ||
		typeof hourlyMinimumRangePercent !== "number" ||
		typeof minimumMarketCapMillions !== "number" ||
		Object.keys(
			validateMarketScanCriteria({
				hourlyMinimumRangePercent,
				hourlyPercentile,
				hourlyPeriod,
				minimumMarketCapMillions,
				minimumRangePercent,
				percentile,
				period,
			}),
		).length > 0
	) {
		return instrumentAnalysisCriteriaToSearch(defaultMarketScanCriteria);
	}

	return {
		hourly_minimum_range_percent: hourlyMinimumRangePercent,
		hourly_percentile: hourlyPercentile,
		hourly_period: hourlyPeriod,
		minimum_market_cap_millions: minimumMarketCapMillions,
		minimum_range_percent: minimumRangePercent,
		percentile,
		period,
	};
}

export function instrumentAnalysisCriteriaToSearch(
	criteria: MarketScanCriteria,
): InstrumentAnalysisSearch {
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

export function instrumentAnalysisCriteriaFromSearch(
	search: InstrumentAnalysisSearch,
): MarketScanCriteria {
	return {
		hourlyMinimumRangePercent: search.hourly_minimum_range_percent,
		hourlyPercentile: search.hourly_percentile,
		hourlyPeriod: search.hourly_period,
		minimumMarketCapMillions: search.minimum_market_cap_millions,
		minimumRangePercent: search.minimum_range_percent,
		percentile: search.percentile,
		period: search.period,
	};
}
