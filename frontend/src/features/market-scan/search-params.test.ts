import { describe, expect, it } from "vitest";
import { defaultMarketScanCriteria } from "./pipeline";
import {
	marketScanCriteriaFromSearch,
	marketScanCriteriaToSearch,
	parseMarketScanSearch,
} from "./search-params";

const criteria = {
	...defaultMarketScanCriteria,
	period: 12,
	percentile: 70,
	minimumRangePercent: 5.5,
	hourlyPeriod: 72,
	hourlyPercentile: 90,
	hourlyMinimumRangePercent: 2.5,
	minimumMarketCapMillions: 1,
};
const search = {
	period: 12,
	percentile: 70,
	minimum_range_percent: 5.5,
	hourly_period: 72,
	hourly_percentile: 90,
	hourly_minimum_range_percent: 2.5,
	minimum_market_cap_millions: 1,
};

describe("Market Scan search parameters", () => {
	it("round-trips independent daily and hourly settings without a unit mode", () => {
		expect(marketScanCriteriaToSearch(criteria)).toEqual(search);
		expect(marketScanCriteriaFromSearch(parseMarketScanSearch(search))).toEqual(
			criteria,
		);
	});
	it("discards unit rather than using it to choose a mode", () => {
		expect(parseMarketScanSearch({ ...search, unit: "hours" })).toEqual(search);
	});
	it("omits Market Cap when absent from the URL", () => {
		expect(
			marketScanCriteriaFromSearch(
				parseMarketScanSearch({
					...search,
					minimum_market_cap_millions: undefined,
				}),
			),
		).toEqual({ ...criteria, minimumMarketCapMillions: 0 });
	});
	it("treats absent and old single-unit URLs as uncommitted", () => {
		expect(parseMarketScanSearch({})).toEqual({});
		expect(
			parseMarketScanSearch({
				period: 30,
				percentile: 80,
				minimum_range_percent: 3,
				unit: "hours",
			}),
		).toEqual({});
		expect(marketScanCriteriaFromSearch({})).toBeUndefined();
	});
	it.each([
		"period",
		"percentile",
		"minimum_range_percent",
		"hourly_period",
		"hourly_percentile",
		"hourly_minimum_range_percent",
	])("rejects missing or invalid %s", (field) => {
		for (const value of [
			undefined,
			"30",
			-1,
			Number.NaN,
			Number.POSITIVE_INFINITY,
		]) {
			expect(parseMarketScanSearch({ ...search, [field]: value })).toEqual({});
		}
	});
	it("accepts separate daily and hourly boundaries", () => {
		const boundaries = {
			...search,
			period: 3650,
			percentile: 0,
			minimum_range_percent: 0,
			hourly_period: 87600,
			hourly_percentile: 100,
			hourly_minimum_range_percent: 0,
			minimum_market_cap_millions: 0,
		};
		expect(parseMarketScanSearch(boundaries)).toEqual(boundaries);
		expect(parseMarketScanSearch({ ...boundaries, period: 3651 })).toEqual({});
		expect(
			parseMarketScanSearch({ ...boundaries, hourly_period: 87601 }),
		).toEqual({});
		expect(
			parseMarketScanSearch({ ...boundaries, minimum_market_cap_millions: -1 }),
		).toEqual({});
	});
});
