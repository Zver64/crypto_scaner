import { describe, expect, it } from "vitest";
import {
	formatUtcCoverageDate,
	hasRequiredInstrumentAnalysisEvaluations,
} from "./presentation";

describe("formatUtcCoverageDate", () => {
	it("renders the UTC calendar date without local-time conversion", () => {
		expect(formatUtcCoverageDate("2026-07-05T00:00:00Z")).toBe(
			"Jul 5, 2026 UTC",
		);
		expect(formatUtcCoverageDate("2026-08-03T23:59:59Z")).toBe(
			"Aug 3, 2026 UTC",
		);
	});
});

describe("hasRequiredInstrumentAnalysisEvaluations", () => {
	const volatility = (matched: boolean) => ({
		candle_count: 30,
		from: "2026-07-05T00:00:00Z",
		key: "volatility",
		label: "Volatility",
		matched,
		metrics: { range_percent: 4 },
		name: "volatility",
		to: "2026-08-03T00:00:00Z",
	});

	it("requires Market Cap when the enabled volatility criterion matched", () => {
		expect(
			hasRequiredInstrumentAnalysisEvaluations([volatility(true)], true),
		).toBe(false);
	});

	it("accepts a missing Market Cap evaluation after failed volatility", () => {
		expect(
			hasRequiredInstrumentAnalysisEvaluations([volatility(false)], true),
		).toBe(true);
	});

	it("accepts a missing Market Cap evaluation when disabled", () => {
		expect(
			hasRequiredInstrumentAnalysisEvaluations([volatility(true)], false),
		).toBe(true);
	});
});
