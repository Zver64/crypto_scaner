import { describe, expect, it } from "vitest";
import {
	buildTopCoinsCriteria,
	buildTopCoinsScanCriteria,
} from "@/features/top-coins/top-coins";

describe("Top Market Cap criteria", () => {
	it("uses non-default periods and percentiles without introducing filters", () => {
		const settings = {
			hourlyPercentile: 95,
			hourlyPeriod: 100,
			percentile: 90,
			period: 60,
		};

		expect(buildTopCoinsScanCriteria(settings)).toEqual({
			...settings,
			dailyMaxRsi: 100,
			hourlyMinimumRangePercent: 0,
			minimumMarketCapMillions: 0,
			minimumRangePercent: 0,
			weeklyMaxRsi: 100,
		});
		expect(
			buildTopCoinsCriteria(settings).map(({ parameters }) => parameters),
		).toEqual([
			{
				minimum_range_percent: 0,
				percentile: 90,
				period: 60,
				unit: "days",
			},
			{
				minimum_range_percent: 0,
				percentile: 95,
				period: 100,
				unit: "hours",
			},
			{ min_market_cap_usd: 0 },
		]);
	});
});
