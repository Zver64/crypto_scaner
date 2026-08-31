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
	const dailyVolatility = (matched: boolean) => ({
		candle_count: 30,
		from: "2026-07-05T00:00:00Z",
		key: "daily_volatility",
		label: "Daily Volatility",
		matched,
		metrics: { range_percent: 4 },
		name: "volatility",
		to: "2026-08-03T00:00:00Z",
	});
	const hourlyVolatility = (matched: boolean) => ({
		...dailyVolatility(matched),
		candle_count: 60,
		key: "hourly_volatility",
		label: "Hourly Volatility",
	});

	it("requires Hourly Volatility after a matching Daily Volatility", () => {
		expect(
			hasRequiredInstrumentAnalysisEvaluations([dailyVolatility(true)], false),
		).toBe(false);
	});

	it("accepts later evaluations omitted after a failed Daily Volatility", () => {
		expect(
			hasRequiredInstrumentAnalysisEvaluations([dailyVolatility(false)], true),
		).toBe(true);
	});

	it("requires Market Cap only after both matching volatility evaluations", () => {
		expect(
			hasRequiredInstrumentAnalysisEvaluations(
				[dailyVolatility(true), hourlyVolatility(true)],
				true,
			),
		).toBe(false);
	});
});
