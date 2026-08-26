import { describe, expect, it } from "vitest";
import {
	marketScanCriteriaFromSearch,
	marketScanCriteriaToSearch,
	parseMarketScanSearch,
} from "./search-params";

describe("Market Scan search parameters", () => {
	it("round-trips committed criteria without renaming backend parameters", () => {
		const search = marketScanCriteriaToSearch({
			minimumRangePercent: 3.5,
			minimumMarketCapMillions: 1,
			percentile: 80,
			period: 30,
			unit: "days",
		});

		expect(search).toEqual({
			minimum_range_percent: 3.5,
			minimum_market_cap_millions: 1,
			percentile: 80,
			period: 30,
			unit: "days",
		});
		expect(marketScanCriteriaFromSearch(parseMarketScanSearch(search))).toEqual(
			{
				minimumRangePercent: 3.5,
				minimumMarketCapMillions: 1,
				percentile: 80,
				period: 30,
				unit: "days",
			},
		);
	});

	it("preserves legacy criteria when Market Cap is absent", () => {
		expect(
			marketScanCriteriaFromSearch(
				parseMarketScanSearch({
					minimum_range_percent: 3.5,
					percentile: 75,
					period: 60,
					unit: "hours",
				}),
			),
		).toEqual({
			minimumMarketCapMillions: 0,
			minimumRangePercent: 3.5,
			percentile: 75,
			period: 60,
			unit: "hours",
		});
	});

	it("treats absent, partial, and invalid URL state as uncommitted", () => {
		expect(parseMarketScanSearch({})).toEqual({});
		expect(
			parseMarketScanSearch({ percentile: 80, period: 30, unit: "days" }),
		).toEqual({});
		expect(
			parseMarketScanSearch({
				minimum_range_percent: -1,
				minimum_market_cap_millions: -1,
				percentile: 101,
				period: 0,
				unit: "days",
			}),
		).toEqual({});
	});

	it("accepts supported URL boundaries", () => {
		expect(
			parseMarketScanSearch({
				minimum_range_percent: 0,
				minimum_market_cap_millions: 0,
				percentile: 100,
				period: 87600,
				unit: "hours",
			}),
		).toEqual({
			minimum_range_percent: 0,
			minimum_market_cap_millions: 0,
			percentile: 100,
			period: 87600,
			unit: "hours",
		});
	});
});
