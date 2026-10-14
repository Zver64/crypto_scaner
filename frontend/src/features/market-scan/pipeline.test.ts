import { expect, it } from "vitest";

import {
	criterionSelections,
	defaultMarketScanCriteria,
	volatilityCriterionSelections,
} from "@/features/market-scan/pipeline";

it("keeps independent settings and includes Market Cap at zero", () => {
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
		{
			key: "market_cap",
			label: "Market Cap",
			name: "market_cap",
			parameters: { min_market_cap_usd: 0 },
		},
	]);
});

it("selects only the volatility criteria for the coin page", () => {
	expect(
		volatilityCriterionSelections(defaultMarketScanCriteria).map(
			({ key }) => key,
		),
	).toEqual(["daily_volatility", "hourly_volatility"]);
});
