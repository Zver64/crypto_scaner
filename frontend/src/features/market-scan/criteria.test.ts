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
				periodDays: 1,
			}),
		).toEqual({});
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: 0.1,
				percentile: 100,
				periodDays: 3650,
			}),
		).toEqual({});
	});

	it("rejects fractional or out-of-range integer fields", () => {
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: 3,
				percentile: 80.5,
				periodDays: 30.5,
			}),
		).toEqual({
			percentile: "Range percentile must be a whole number",
			periodDays: "Analysis period must be a whole number",
		});
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: -0.1,
				percentile: 101,
				periodDays: 0,
			}),
		).toEqual({
			minimumRangePercent: "Minimum range must be zero or greater",
			percentile: "Range percentile must be between 0 and 100",
			periodDays: "Analysis period must be between 1 and 3650 days",
		});
	});

	it("rejects empty and non-numeric draft values", () => {
		expect(
			validateMarketScanCriteria({
				minimumRangePercent: "",
				percentile: "",
				periodDays: "",
			}),
		).toEqual({
			minimumRangePercent: "Minimum range is required",
			percentile: "Range percentile is required",
			periodDays: "Analysis period is required",
		});
	});
});
