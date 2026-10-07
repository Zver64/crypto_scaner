import { describe, expect, it } from "vitest";
import {
	displayedPercentSign,
	formatRangePercent,
} from "@/utils/range-percent";

describe("formatRangePercent", () => {
	it.each([
		[0, "0%"],
		[0.0045678, "0.004568%"],
		[0.99995, "1%"],
		[1.0005, "1.001%"],
		[9.43812, "9.438%"],
		[1234.5, "1,235%"],
	])("formats %s keeping the integer part as %s", (value, expected) => {
		expect(formatRangePercent(value)).toBe(expected);
	});

	it("limits the fraction digits when asked", () => {
		expect(formatRangePercent(7.273, 2)).toBe("7.27%");
		expect(formatRangePercent(-0.004, 2)).toBe("0%");
	});
});

describe("displayedPercentSign", () => {
	it.each([
		[-0.004, 2, 0],
		[0, undefined, 0],
		[-0.004, undefined, -1],
		[0.006, 2, 1],
		[-7.27, 2, -1],
	])("signs %s with %s fraction digits as %s", (value, digits, expected) => {
		expect(displayedPercentSign(value, digits)).toBe(expected);
	});
});
