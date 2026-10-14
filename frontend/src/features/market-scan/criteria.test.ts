import { expect, it } from "vitest";

import {
	criterionSelections,
	volatilityCriterionSelection,
	volatilityEvaluation,
} from "@/features/market-scan/criteria";

it("adapts volatility form criteria and safely extracts its evaluation", () => {
	expect(
		volatilityCriterionSelection({
			minimumRangePercent: 3.5,
			minimumMarketCapMillions: 0,
			percentile: 80,
			period: 30,
			unit: "days",
		}),
	).toEqual({
		key: "volatility",
		label: "Volatility",
		name: "volatility",
		parameters: {
			minimum_range_percent: 3.5,
			percentile: 80,
			period: 30,
			unit: "days",
		},
	});
	expect(
		volatilityEvaluation([
			{
				candle_count: 30,
				from: "2026-08-01T00:00:00Z",
				key: "volatility",
				label: "Volatility",
				matched: true,
				metrics: { range_percent: 4 },
				name: "volatility",
				to: "2026-08-02T00:00:00Z",
			},
		]),
	).toEqual({
		candleCount: 30,
		from: "2026-08-01T00:00:00Z",
		matched: true,
		rangePercent: 4,
		to: "2026-08-02T00:00:00Z",
	});
	expect(volatilityEvaluation([])).toBeUndefined();
});

it("always sends Market Cap after volatility, including when its minimum is zero", () => {
	const criteria = {
		minimumMarketCapMillions: 1,
		minimumRangePercent: 3.5,
		percentile: 80,
		period: 30,
		unit: "days" as const,
	};

	expect(criterionSelections(criteria)).toEqual([
		volatilityCriterionSelection(criteria),
		{
			key: "market_cap",
			label: "Market Cap",
			name: "market_cap",
			parameters: { min_market_cap_usd: 1_000_000 },
		},
	]);
	expect(
		criterionSelections({ ...criteria, minimumMarketCapMillions: 0 }),
	).toEqual([
		volatilityCriterionSelection({ ...criteria, minimumMarketCapMillions: 0 }),
		{
			key: "market_cap",
			label: "Market Cap",
			name: "market_cap",
			parameters: { min_market_cap_usd: 0 },
		},
	]);
});
