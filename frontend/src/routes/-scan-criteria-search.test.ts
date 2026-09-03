import { describe, expect, it } from "vitest";
import { defaultMarketScanCriteria } from "@/features/market-scan/pipeline";
import {
	parseOptionalScanCriteriaSearch,
	parseRequiredScanCriteriaSearch,
	scanCriteriaFromSearch,
	scanCriteriaToSearch,
} from "@/routes/-scan-criteria-search";

const criteria = {
	...defaultMarketScanCriteria,
	hourlyMinimumRangePercent: 2.5,
	hourlyPercentile: 90,
	hourlyPeriod: 72,
	minimumMarketCapMillions: 1,
	minimumRangePercent: 5.5,
	percentile: 70,
	period: 12,
};

const search = {
	hourly_minimum_range_percent: 2.5,
	hourly_percentile: 90,
	hourly_period: 72,
	minimum_market_cap_millions: 1,
	minimum_range_percent: 5.5,
	percentile: 70,
	period: 12,
};

describe("scan criteria URL state", () => {
	it("round-trips independent daily and hourly criteria", () => {
		expect(scanCriteriaToSearch(criteria)).toEqual(search);
		expect(scanCriteriaFromSearch(search)).toEqual(criteria);
	});

	it("keeps an absent market cap criterion disabled", () => {
		expect(
			scanCriteriaFromSearch({
				...search,
				minimum_market_cap_millions: undefined,
			}),
		).toEqual({ ...criteria, minimumMarketCapMillions: 0 });
	});

	it.each([
		[
			"period",
			[undefined, "30", -1, 0.5, Number.NaN, Number.POSITIVE_INFINITY],
		],
		[
			"percentile",
			[undefined, "30", -1, 0.5, Number.NaN, Number.POSITIVE_INFINITY],
		],
		[
			"minimum_range_percent",
			[undefined, "30", -1, Number.NaN, Number.POSITIVE_INFINITY],
		],
		[
			"hourly_period",
			[undefined, "30", -1, 0.5, Number.NaN, Number.POSITIVE_INFINITY],
		],
		[
			"hourly_percentile",
			[undefined, "30", -1, 0.5, Number.NaN, Number.POSITIVE_INFINITY],
		],
		[
			"hourly_minimum_range_percent",
			[undefined, "30", -1, Number.NaN, Number.POSITIVE_INFINITY],
		],
	] as const)("rejects missing or invalid %s", (field, values) => {
		for (const value of [...values]) {
			const invalidSearch = { ...search, [field]: value };
			expect(parseOptionalScanCriteriaSearch(invalidSearch)).toEqual({});
			expect(parseRequiredScanCriteriaSearch(invalidSearch)).toEqual(
				scanCriteriaToSearch(defaultMarketScanCriteria),
			);
		}
	});

	it("accepts fractional daily and hourly ranges", () => {
		expect(
			parseOptionalScanCriteriaSearch({
				...search,
				hourly_minimum_range_percent: 0.5,
				minimum_range_percent: 0.5,
			}),
		).toMatchObject({
			hourly_minimum_range_percent: 0.5,
			minimum_range_percent: 0.5,
		});
	});

	it("accepts valid daily and hourly limits and rejects values past them", () => {
		const boundaries = {
			...search,
			hourly_minimum_range_percent: 0,
			hourly_percentile: 100,
			hourly_period: 87600,
			minimum_market_cap_millions: 0,
			minimum_range_percent: 0,
			percentile: 0,
			period: 3650,
		};
		expect(parseOptionalScanCriteriaSearch(boundaries)).toEqual(boundaries);
		expect(
			parseOptionalScanCriteriaSearch({ ...boundaries, period: 3651 }),
		).toEqual({});
		expect(
			parseOptionalScanCriteriaSearch({ ...boundaries, hourly_period: 87601 }),
		).toEqual({});
	});
});
