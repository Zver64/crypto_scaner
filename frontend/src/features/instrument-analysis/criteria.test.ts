import { describe, expect, it } from "vitest";
import {
	defaultInstrumentAnalysisCriteria,
	validateInstrumentAnalysisCriteria,
} from "./criteria";

describe("validateInstrumentAnalysisCriteria", () => {
	it("accepts defaults and supported boundaries", () => {
		expect(
			validateInstrumentAnalysisCriteria(defaultInstrumentAnalysisCriteria),
		).toEqual({});
		expect(
			validateInstrumentAnalysisCriteria({
				percentile: 0,
				period: 1,
				unit: "days",
			}),
		).toEqual({});
		expect(
			validateInstrumentAnalysisCriteria({
				percentile: 100,
				period: 87600,
				unit: "hours",
			}),
		).toEqual({});
	});

	it("rejects empty, fractional, and out-of-range values", () => {
		expect(
			validateInstrumentAnalysisCriteria({
				percentile: "",
				period: "",
				unit: "days",
			}),
		).toEqual({
			percentile: "Range percentile is required",
			period: "Analysis period is required",
		});
		expect(
			validateInstrumentAnalysisCriteria({
				percentile: 80.5,
				period: 30.5,
				unit: "days",
			}),
		).toEqual({
			percentile: "Range percentile must be a whole number",
			period: "Analysis period must be a whole number",
		});
		expect(
			validateInstrumentAnalysisCriteria({
				percentile: 101,
				period: 0,
				unit: "days",
			}),
		).toEqual({
			percentile: "Range percentile must be between 0 and 100",
			period: "Analysis period must be a whole number between 1 and 3650 days",
		});
	});
});
