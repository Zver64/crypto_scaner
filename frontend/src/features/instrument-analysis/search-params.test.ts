import { describe, expect, it } from "vitest";
import {
	instrumentAnalysisCriteriaFromSearch,
	instrumentAnalysisCriteriaToSearch,
	parseInstrumentAnalysisSearch,
} from "./search-params";

describe("Instrument Analysis search parameters", () => {
	it("round-trips committed period and percentile", () => {
		const search = instrumentAnalysisCriteriaToSearch({
			percentile: 75,
			periodDays: 60,
		});

		expect(search).toEqual({ percentile: 75, period_days: 60 });
		expect(
			instrumentAnalysisCriteriaFromSearch(
				parseInstrumentAnalysisSearch(search),
			),
		).toEqual({ percentile: 75, periodDays: 60 });
	});

	it("falls back to supported defaults for absent, partial, or invalid URL state", () => {
		for (const search of [
			{},
			{ percentile: 75 },
			{ percentile: 101, period_days: 0 },
			{ percentile: 80.5, period_days: 30.5 },
		]) {
			expect(parseInstrumentAnalysisSearch(search)).toEqual({
				percentile: 80,
				period_days: 30,
			});
		}
	});
});
