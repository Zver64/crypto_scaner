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
			percentile: 80,
			period: 30,
			unit: "days",
		});

		expect(search).toEqual({
			minimum_range_percent: 3.5,
			percentile: 80,
			period: 30,
			unit: "days",
		});
		expect(marketScanCriteriaFromSearch(parseMarketScanSearch(search))).toEqual(
			{
				minimumRangePercent: 3.5,
				percentile: 80,
				period: 30,
				unit: "days",
			},
		);
	});

	it("treats absent, partial, and invalid URL state as uncommitted", () => {
		expect(parseMarketScanSearch({})).toEqual({});
		expect(
			parseMarketScanSearch({ percentile: 80, period: 30, unit: "days" }),
		).toEqual({});
		expect(
			parseMarketScanSearch({
				minimum_range_percent: -1,
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
				percentile: 100,
				period: 87600,
				unit: "hours",
			}),
		).toEqual({
			minimum_range_percent: 0,
			percentile: 100,
			period: 87600,
			unit: "hours",
		});
	});
});
