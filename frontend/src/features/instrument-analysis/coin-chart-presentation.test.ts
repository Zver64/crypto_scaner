import { expect, it } from "vitest";
import {
	formatCoinChartTime,
	nextCoinCandleOpen,
	rangeReadout,
} from "@/features/instrument-analysis/coin-chart-presentation";

it("formats the coin's UTC timeframe labels", () => {
	expect(formatCoinChartTime("2026-08-26T23:00:00Z", "1h")).toBe(
		"Aug 26, 23:00 UTC",
	);
	expect(formatCoinChartTime("2026-08-26T23:00:00Z", "1w")).toContain(
		"Week of",
	);
});

it("advances each configured interval and formats the coin range metric", () => {
	expect(nextCoinCandleOpen("2026-01-01T00:00:00Z", "1M")).toBe(
		"2026-02-01T00:00:00.000Z",
	);
	expect(nextCoinCandleOpen("2026-08-26T23:00:00Z", "1h")).toBe(
		"2026-08-27T00:00:00.000Z",
	);
	expect(rangeReadout.format({ open: 1, high: 2, low: 0.5, close: 1.5 })).toBe(
		"150%",
	);
});
