import { describe, expect, it } from "vitest";
import {
	defaultMarketScanCriteria,
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
				percentile: 101,
				period: 0,
				unit: "days",
			}),
		).toEqual({
			minimumRangePercent: "Minimum range must be zero or greater",
			percentile: "Range percentile must be between 0 and 100",
			period: "Analysis period must be a whole number between 1 and 3650 days",
		});
	});

	it("requires a numeric minimum range", () => {
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: "",
				percentile: 80,
				period: 30,
				unit: "days",
			}),
		).toEqual({
			minimumRangePercent: "Minimum range is required",
		});
	});
});

it("adapts percentile form criteria and safely extracts its evaluation", () => {
	expect(
		percentileCriterionSelection({
			minimumRangePercent: 3.5,
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
