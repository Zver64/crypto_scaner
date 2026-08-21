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
			period: 60,
			unit: "hours",
		});

		expect(search).toEqual({ percentile: 75, period: 60, unit: "hours" });
		expect(
			instrumentAnalysisCriteriaFromSearch(
				parseInstrumentAnalysisSearch(search),
			),
		).toEqual({ percentile: 75, period: 60, unit: "hours" });
	});

	it("falls back to supported defaults for absent, partial, or invalid URL state", () => {
		for (const search of [
			{},
			{ percentile: 75 },
			{ percentile: 101, period: 0, unit: "days" },
			{ percentile: 80.5, period: 30.5, unit: "days" },
		]) {
			expect(parseInstrumentAnalysisSearch(search)).toEqual({
				percentile: 80,
				period: 30,
				unit: "days",
			});
		}
	});
});
