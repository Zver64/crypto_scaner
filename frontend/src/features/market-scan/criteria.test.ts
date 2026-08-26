import { describe, expect, it } from "vitest";
import {
	criterionSelections,
	defaultMarketScanCriteria,
	marketCapEvaluation,
	percentileCriterionSelection,
	percentileEvaluation,
	validateMarketScanCriteria,
} from "./criteria";

describe("validateMarketScanCriteria", () => {
	it("accepts the default minimum range and zero", () => {
		expect(validateMarketScanCriteria(defaultMarketScanCriteria)).toEqual({});
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: 0,
				minimumMarketCapMillions: 0,
				percentile: 80,
				period: 30,
				unit: "days",
			}),
		).toEqual({});
	});

	it("rejects a negative minimum range", () => {
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: -0.1,
				minimumMarketCapMillions: 0,
				percentile: 80,
				period: 30,
				unit: "days",
			}),
		).toEqual({
			minimumRangePercent: "Minimum range must be zero or greater",
		});
	});

	it("combines shared analysis and scan-specific validation errors", () => {
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: -0.1,
				minimumMarketCapMillions: -1,
				percentile: 101,
				period: 0,
				unit: "days",
			}),
		).toEqual({
			minimumMarketCapMillions: "Minimum market cap must be zero or greater",
			minimumRangePercent: "Minimum range must be zero or greater",
			percentile: "Range percentile must be between 0 and 100",
			period: "Analysis period must be a whole number between 1 and 3650 days",
		});
	});

	it("requires a numeric minimum range", () => {
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: "",
				minimumMarketCapMillions: "",
				percentile: 80,
				period: 30,
				unit: "days",
			}),
		).toEqual({
			minimumMarketCapMillions: "Minimum market cap is required",
			minimumRangePercent: "Minimum range is required",
		});
	});
});

it("adapts percentile form criteria and safely extracts its evaluation", () => {
	expect(
		percentileCriterionSelection({
			minimumRangePercent: 3.5,
			minimumMarketCapMillions: 0,
			percentile: 80,
			period: 30,
			unit: "days",
		}),
	).toEqual({
		name: "percentile",
		parameters: {
			minimum_range_percent: 3.5,
			percentile: 80,
			period: 30,
			unit: "days",
		},
	});
	expect(
		percentileEvaluation([
			{
				candle_count: 30,
				from: "2026-08-01T00:00:00Z",
				matched: true,
				metrics: { range_percent: 4 },
				name: "percentile",
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
	expect(percentileEvaluation([])).toBeUndefined();
});

it("sends Market Cap after percentile only when it is positive", () => {
	const criteria = {
		minimumMarketCapMillions: 1,
		minimumRangePercent: 3.5,
		percentile: 80,
		period: 30,
		unit: "days" as const,
	};

	expect(criterionSelections(criteria)).toEqual([
		percentileCriterionSelection(criteria),
		{ name: "market_cap", parameters: { min_market_cap_usd: 1_000_000 } },
	]);
	expect(
		criterionSelections({ ...criteria, minimumMarketCapMillions: 0 }),
	).toEqual([
		percentileCriterionSelection({ ...criteria, minimumMarketCapMillions: 0 }),
	]);
});

it("extracts a Market Cap evaluation without candle coverage", () => {
	expect(
		marketCapEvaluation([
			{
				candle_count: 0,
				from: "0001-01-01T00:00:00Z",
				matched: true,
				metrics: { market_cap_usd: 1_234_567 },
				name: "market_cap",
				to: "0001-01-01T00:00:00Z",
			},
		]),
	).toEqual({ marketCapUsd: 1_234_567, matched: true });
});
