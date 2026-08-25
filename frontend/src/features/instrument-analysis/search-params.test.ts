import { describe, expect, it } from "vitest";
import {
	instrumentAnalysisCriteriaFromSearch,
	instrumentAnalysisCriteriaToSearch,
	parseInstrumentAnalysisSearch,
} from "./search-params";

describe("Instrument Analysis search parameters", () => {
	it("round-trips committed percentile parameters including minimum range", () => {
		const search = instrumentAnalysisCriteriaToSearch({
			minimumRangePercent: 3.5,
			percentile: 75,
			period: 60,
			unit: "hours",
		});

		expect(search).toEqual({
			minimum_range_percent: 3.5,
			percentile: 75,
			period: 60,
			unit: "hours",
		});
		expect(
			instrumentAnalysisCriteriaFromSearch(
				parseInstrumentAnalysisSearch(search),
			),
		).toEqual({
			minimumRangePercent: 3.5,
			percentile: 75,
			period: 60,
			unit: "hours",
		});
	});

	it("falls back to supported defaults for absent, partial, or invalid URL state", () => {
		for (const search of [
			{},
			{ percentile: 75 },
			{ minimum_range_percent: 3, percentile: 101, period: 0, unit: "days" },
			{
				minimum_range_percent: -1,
				percentile: 80.5,
				period: 30.5,
				unit: "days",
			},
		]) {
			expect(parseInstrumentAnalysisSearch(search)).toEqual({
				minimum_range_percent: 3,
				percentile: 80,
				period: 30,
				unit: "days",
			});
		}
	});
});
