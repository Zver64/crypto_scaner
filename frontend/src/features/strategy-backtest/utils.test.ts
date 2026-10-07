import { describe, expect, it } from "vitest";
import {
	createEquityData,
	formatFractionPercent,
} from "@/features/strategy-backtest/utils";

describe("formatFractionPercent", () => {
	it("formats fractions as percentages and a missing value as a dash", () => {
		expect(formatFractionPercent(-0.0123)).toBe("-1.23%");
		expect(formatFractionPercent(null)).toBe("—");
	});
});

describe("createEquityData", () => {
	it("starts at zero on the first evaluated candle and shows compounded net profit", () => {
		expect(
			createEquityData("2026-08-01T00:00:00Z", [
				{ equity: 1.1, time: "2026-08-03T00:00:00Z" },
				{ equity: 0.99, time: "2026-08-05T00:00:00Z" },
			]).map(({ time, value }) => [time, Number(value.toFixed(6))]),
		).toEqual([
			[Date.parse("2026-08-01T00:00:00Z") / 1_000, 0],
			[Date.parse("2026-08-03T00:00:00Z") / 1_000, 10],
			[Date.parse("2026-08-05T00:00:00Z") / 1_000, -1],
		]);
	});
});
