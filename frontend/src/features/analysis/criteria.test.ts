import { describe, expect, it } from "vitest";
import { defaultAnalysisCriteria, validateAnalysisCriteria } from "./criteria";

describe("analysis criteria", () => {
	it("accepts supported day and hour boundaries", () => {
		expect(validateAnalysisCriteria(defaultAnalysisCriteria)).toEqual({});
		expect(
			validateAnalysisCriteria({ percentile: 0, period: 1, unit: "days" }),
		).toEqual({});
		expect(
			validateAnalysisCriteria({
				percentile: 100,
				period: 2000,
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
				period: 2001,
				unit: "days",
			}),
		).toEqual({
			percentile: "Range percentile must be between 0 and 100",
			period: "Analysis period must be a whole number between 1 and 2000 days",
		});
		expect(
			validateAnalysisCriteria({
				percentile: 80.5,
				period: 2001,
				unit: "hours",
			}),
		).toEqual({
			percentile: "Range percentile must be a whole number",
			period: "Analysis period must be a whole number between 1 and 2000 hours",
		});
	});
});
