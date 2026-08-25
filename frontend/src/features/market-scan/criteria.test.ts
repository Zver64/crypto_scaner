import { describe, expect, it } from "vitest";
import {
	defaultMarketScanCriteria,
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
