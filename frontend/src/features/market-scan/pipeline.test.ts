import { expect, it } from "vitest";
import {
	criterionSelections,
	defaultMarketScanCriteria,
	validateMarketScanCriteria,
} from "./pipeline";

it("composes mandatory daily and hourly volatility before enabled Market Cap", () => {
	expect(criterionSelections(defaultMarketScanCriteria)).toEqual([
		{
			key: "daily_volatility",
			label: "Daily Volatility",
			name: "volatility",
			parameters: {
				unit: "days",
				period: 30,
				percentile: 80,
				minimum_range_percent: 5,
			},
		},
		{
			key: "hourly_volatility",
			label: "Hourly Volatility",
			name: "volatility",
			parameters: {
				unit: "hours",
				period: 60,
				percentile: 80,
				minimum_range_percent: 2,
			},
		},
		{
			key: "market_cap",
			label: "Market Cap",
			name: "market_cap",
			parameters: { min_market_cap_usd: 500_000_000 },
		},
	]);
});

it("keeps independent settings and omits only Market Cap at zero", () => {
	expect(
		criterionSelections({
			...defaultMarketScanCriteria,
			period: 12,
			percentile: 55,
			minimumRangePercent: 0,
			hourlyPeriod: 72,
			hourlyPercentile: 95,
			hourlyMinimumRangePercent: 1.5,
			minimumMarketCapMillions: 0,
		}),
	).toEqual([
		{
			key: "daily_volatility",
			label: "Daily Volatility",
			name: "volatility",
			parameters: {
				unit: "days",
				period: 12,
				percentile: 55,
				minimum_range_percent: 0,
			},
		},
		{
			key: "hourly_volatility",
			label: "Hourly Volatility",
			name: "volatility",
			parameters: {
				unit: "hours",
				period: 72,
				percentile: 95,
				minimum_range_percent: 1.5,
			},
		},
	]);
});

it("validates both mandatory volatility instances independently", () => {
	expect(validateMarketScanCriteria(defaultMarketScanCriteria)).toEqual({});
	expect(
		validateMarketScanCriteria({
			...defaultMarketScanCriteria,
			period: 3651,
			percentile: 101,
			minimumRangePercent: -1,
			hourlyPeriod: 87601,
			hourlyPercentile: -1,
			hourlyMinimumRangePercent: "",
			minimumMarketCapMillions: -1,
		}),
	).toEqual({
		period: "Analysis period must be a whole number between 1 and 3650 days",
		percentile: "Range percentile must be between 0 and 100",
		minimumRangePercent: "Minimum range must be zero or greater",
		hourlyPeriod:
			"Analysis period must be a whole number between 1 and 87600 hours",
		hourlyPercentile: "Range percentile must be between 0 and 100",
		hourlyMinimumRangePercent: "Minimum range is required",
		minimumMarketCapMillions: "Minimum market cap must be zero or greater",
	});
});

it.each([
	"period",
	"percentile",
	"minimumRangePercent",
	"hourlyPeriod",
	"hourlyPercentile",
	"hourlyMinimumRangePercent",
	"minimumMarketCapMillions",
] as const)("rejects missing and non-numeric %s", (field) => {
	for (const value of ["", "2", Number.NaN, Number.POSITIVE_INFINITY]) {
		expect(
			validateMarketScanCriteria({
				...defaultMarketScanCriteria,
				[field]: value,
			}),
		).toHaveProperty(field);
	}
});
