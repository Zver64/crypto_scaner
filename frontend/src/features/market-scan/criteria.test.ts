import { describe, expect, it } from "vitest";
import {
	defaultMarketScanCriteria,
	validateMarketScanCriteria,
} from "./criteria";

describe("validateMarketScanCriteria", () => {
	it("accepts the defaults and supported boundary values", () => {
		expect(validateMarketScanCriteria(defaultMarketScanCriteria)).toEqual({});
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: 0,
				percentile: 0,
				period: 1,
				unit: "days",
			}),
		).toEqual({});
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: 0.1,
				percentile: 100,
				period: 87600,
				unit: "hours",
			}),
		).toEqual({});
	});

	it("rejects fractional or out-of-range integer fields", () => {
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: 3,
				percentile: 80.5,
				period: 30.5,
				unit: "days",
			}),
		).toEqual({
			percentile: "Range percentile must be a whole number",
			period: "Analysis period must be a whole number",
		});
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

	it("rejects empty and non-numeric draft values", () => {
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: "",
				percentile: "",
				period: "",
				unit: "days",
			}),
		).toEqual({
			minimumRangePercent: "Minimum range is required",
			percentile: "Range percentile is required",
			period: "Analysis period is required",
		});
	});
});
