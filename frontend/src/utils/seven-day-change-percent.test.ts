import { describe, expect, it } from "vitest";
import { sevenDayChangePercent } from "@/utils/seven-day-change-percent";

describe("sevenDayChangePercent", () => {
	it("calculates the change between the first and last available prices", () => {
		expect(sevenDayChangePercent([null, 100, 120, null])).toBe(20);
		expect(sevenDayChangePercent([100, 80])).toBe(-20);
	});

	it("uses the available period when less than seven days of prices exist", () => {
		expect(sevenDayChangePercent([100])).toBe(0);
		expect(sevenDayChangePercent([null, 100, 110, null])).toBe(10);
	});

	it.each([
		[[], null],
		[[0, 100], null],
	] as const)("returns null for unavailable or invalid history %j", (prices, expected) => {
		expect(sevenDayChangePercent(prices)).toBe(expected);
	});
});
