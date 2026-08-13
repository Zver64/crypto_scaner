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
			validateInstrumentAnalysisCriteria({ percentile: 0, periodDays: 1 }),
		).toEqual({});
		expect(
			validateInstrumentAnalysisCriteria({
				percentile: 100,
				periodDays: 3650,
			}),
		).toEqual({});
	});

	it("rejects empty, fractional, and out-of-range values", () => {
		expect(
			validateInstrumentAnalysisCriteria({ percentile: "", periodDays: "" }),
		).toEqual({
			percentile: "Range percentile is required",
			periodDays: "Analysis period is required",
		});
		expect(
			validateInstrumentAnalysisCriteria({
				percentile: 80.5,
				periodDays: 30.5,
			}),
		).toEqual({
			percentile: "Range percentile must be a whole number",
			periodDays: "Analysis period must be a whole number",
		});
		expect(
			validateInstrumentAnalysisCriteria({ percentile: 101, periodDays: 0 }),
		).toEqual({
			percentile: "Range percentile must be between 0 and 100",
			periodDays: "Analysis period must be between 1 and 3650 days",
		});
	});
});
