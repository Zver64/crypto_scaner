import { describe, expect, it } from "vitest";
import { formatRangePercent } from "@/utils/range-percent";

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
});
