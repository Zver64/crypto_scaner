import {
	defaultMarketScanCriteria,
	type MarketScanCriteria,
	validateMarketScanCriteria,
} from "../market-scan/criteria";

export interface InstrumentAnalysisSearch {
	percentile: number;
	period: number;
	unit: "days" | "hours";
	minimum_range_percent: number;
	minimum_market_cap_millions: number;
}

export function parseInstrumentAnalysisSearch(
	search: Record<string, unknown> | InstrumentAnalysisSearch,
): InstrumentAnalysisSearch {
	const period = search.period;
	const unit = search.unit;
	const percentile = search.percentile;
	const minimumRangePercent = search.minimum_range_percent;
	const minimumMarketCapMillions =
		search.minimum_market_cap_millions === undefined
			? 0
			: search.minimum_market_cap_millions;

	if (
		typeof period !== "number" ||
		typeof percentile !== "number" ||
		typeof minimumRangePercent !== "number" ||
		typeof minimumMarketCapMillions !== "number" ||
		(unit !== "days" && unit !== "hours") ||
		Object.keys(
			validateMarketScanCriteria({
				minimumMarketCapMillions,
				minimumRangePercent,
				percentile,
				period,
				unit,
			}),
		).length > 0
	) {
		return instrumentAnalysisCriteriaToSearch(defaultMarketScanCriteria);
	}

	return {
		minimum_market_cap_millions: minimumMarketCapMillions,
		minimum_range_percent: minimumRangePercent,
		percentile,
		period,
		unit,
	};
}

export function instrumentAnalysisCriteriaToSearch(
	criteria: MarketScanCriteria,
): InstrumentAnalysisSearch {
	return {
		minimum_market_cap_millions: criteria.minimumMarketCapMillions,
		minimum_range_percent: criteria.minimumRangePercent,
		percentile: criteria.percentile,
		period: criteria.period,
		unit: criteria.unit,
	};
}

export function instrumentAnalysisCriteriaFromSearch(
	search: InstrumentAnalysisSearch,
): MarketScanCriteria {
	return {
		minimumMarketCapMillions: search.minimum_market_cap_millions,
		minimumRangePercent: search.minimum_range_percent,
		percentile: search.percentile,
		period: search.period,
		unit: search.unit,
	};
}
