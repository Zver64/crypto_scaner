import { describe, expect, it } from "vitest";
import {
	defaultAnalysisCriteria,
	defaultPeriodForUnit,
	maximumPeriodForUnit,
	validateAnalysisCriteria,
} from "./criteria";

describe("analysis criteria", () => {
	it("provides shared defaults and unit-specific periods", () => {
		expect(defaultAnalysisCriteria).toEqual({
			percentile: 80,
			period: 30,
			unit: "days",
		});
		expect(defaultPeriodForUnit("days")).toBe(30);
		expect(defaultPeriodForUnit("hours")).toBe(60);
		expect(maximumPeriodForUnit("days")).toBe(3650);
		expect(maximumPeriodForUnit("hours")).toBe(87600);
	});

	it("accepts supported day and hour boundaries", () => {
		expect(validateAnalysisCriteria(defaultAnalysisCriteria)).toEqual({});
		expect(
			validateAnalysisCriteria({ percentile: 0, period: 1, unit: "days" }),
		).toEqual({});
		expect(
			validateAnalysisCriteria({
				percentile: 100,
				period: 87600,
				unit: "hours",
			}),
		).toEqual({});
	});

	it("rejects invalid shared criteria", () => {
		expect(
			validateAnalysisCriteria({ percentile: "", period: "", unit: "days" }),
		).toEqual({
			percentile: "Range percentile is required",
			period: "Analysis period is required",
		});
		expect(
			validateAnalysisCriteria({
				percentile: 101,
				period: 3651,
				unit: "days",
			}),
		).toEqual({
			percentile: "Range percentile must be between 0 and 100",
			period: "Analysis period must be a whole number between 1 and 3650 days",
		});
		expect(
			validateAnalysisCriteria({
				percentile: 80.5,
				period: 87601,
				unit: "hours",
			}),
		).toEqual({
			percentile: "Range percentile must be a whole number",
			period:
				"Analysis period must be a whole number between 1 and 87600 hours",
		});
	});
});
