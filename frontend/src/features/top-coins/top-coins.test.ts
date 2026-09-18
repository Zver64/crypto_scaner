import { describe, expect, it } from "vitest";
import type { MarketAnalysisItem } from "@/api/generated/models";
import {
	buildTopCoinsCriteria,
	buildTopCoinsScanCriteria,
	toTopCoinRows,
} from "@/features/top-coins/top-coins";

function item(symbol: string): MarketAnalysisItem {
	return {
		evaluations: [],
		matched: true,
		price_history: [],
		symbol,
	};
}

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
			hourlyMinimumRangePercent: 0,
			minimumMarketCapMillions: 0,
			minimumRangePercent: 0,
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

describe("toTopCoinRows", () => {
	it("preserves the backend result without client-side ranking or limiting", () => {
		const items = Array.from({ length: 11 }, (_, index) =>
			item(`COIN${index}`),
		);

		expect(toTopCoinRows(items).map(({ symbol }) => symbol)).toEqual(
			items.map(({ symbol }) => symbol),
		);
	});
});
