import { describe, expect, it } from "vitest";
import { formatRangePercent } from "@/utils/range-percent";

describe("formatRangePercent", () => {
	it.each([
		[0, "0%"],
		[0.004567, "0.00457%"],
		[0.9995, "1%"],
		[1.005, "1.01%"],
		[9.4381, "9.44%"],
		[1234.5, "1,230%"],
	])("formats %s with three significant digits as %s", (value, expected) => {
		expect(formatRangePercent(value)).toBe(expected);
	});
});
