import { describe, expect, it } from "vitest";
import {
	instrumentAnalysisCriteriaFromSearch,
	instrumentAnalysisCriteriaToSearch,
	parseInstrumentAnalysisSearch,
} from "./search-params";

describe("Instrument Analysis search parameters", () => {
	it("round-trips independent daily and hourly configurations without a unit mode", () => {
		const search = instrumentAnalysisCriteriaToSearch({
			minimumRangePercent: 3.5,
			minimumMarketCapMillions: 1,
			percentile: 75,
			period: 60,
			hourlyMinimumRangePercent: 2.5,
			hourlyPercentile: 90,
			hourlyPeriod: 72,
		});

		expect(search).toEqual({
			hourly_minimum_range_percent: 2.5,
			hourly_percentile: 90,
			hourly_period: 72,
			minimum_range_percent: 3.5,
			minimum_market_cap_millions: 1,
			percentile: 75,
			period: 60,
		});
		expect(
			instrumentAnalysisCriteriaFromSearch(
				parseInstrumentAnalysisSearch(search),
			),
		).toEqual({
			hourlyMinimumRangePercent: 2.5,
			hourlyPercentile: 90,
			hourlyPeriod: 72,
			minimumRangePercent: 3.5,
			minimumMarketCapMillions: 1,
			percentile: 75,
			period: 60,
		});
	});

	it("preserves a disabled Market Cap criterion", () => {
		expect(
			instrumentAnalysisCriteriaFromSearch(
				parseInstrumentAnalysisSearch({
					hourly_minimum_range_percent: 2.5,
					hourly_percentile: 90,
					hourly_period: 72,
					minimum_range_percent: 3.5,
					percentile: 75,
					period: 60,
				}),
			),
		).toEqual({
			hourlyMinimumRangePercent: 2.5,
			hourlyPercentile: 90,
			hourlyPeriod: 72,
			minimumMarketCapMillions: 0,
			minimumRangePercent: 3.5,
			percentile: 75,
			period: 60,
		});
	});

	it("falls back to supported defaults for absent, partial, or invalid URL state", () => {
		for (const search of [
			{},
			{ percentile: 75 },
			{
				hourly_minimum_range_percent: 2,
				hourly_percentile: 80,
				hourly_period: 60,
				minimum_range_percent: 3,
				percentile: 101,
				period: 0,
			},
			{
				hourly_minimum_range_percent: -1,
				hourly_percentile: 80.5,
				hourly_period: 60.5,
				minimum_range_percent: -1,
				percentile: 80.5,
				period: 30.5,
			},
		]) {
			expect(parseInstrumentAnalysisSearch(search)).toEqual({
				hourly_minimum_range_percent: 2,
				hourly_percentile: 80,
				hourly_period: 60,
				minimum_range_percent: 5,
				minimum_market_cap_millions: 500,
				percentile: 80,
				period: 30,
			});
		}
	});
});
