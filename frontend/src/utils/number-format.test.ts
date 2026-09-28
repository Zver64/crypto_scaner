import { describe, expect, it } from "vitest";
import { formatCompactNumber, formatNumber } from "@/utils/number-format";

describe("formatNumber", () => {
	it.each([
		["zero", "0", "0"],
		["small values", "0.0000000123456789", "0.0000000123"],
		["ordinary values", "0.2658481", "0.266"],
		["large values", "12345678.9", "12,300,000"],
		["negative nonzero values", "-0.0000000123456789", "-0.0000000123"],
	])("formats %s with adaptive significant digits", (_, input, expected) => {
		expect(formatNumber(input)).toBe(expected);
	});

	it.each([
		[70, undefined, "70"],
		[1.5, undefined, "1.5"],
		[0.01, undefined, "0.01"],
		["0.0100", undefined, "0.01"],
		["2.5000000", undefined, "2.5"],
		[70, 1, "70"],
		[40.1, 1, "40.1"],
		[0.0101, 10, "0.0101"],
	])("formats %s with fraction limit %s without trailing zeros", (input, maximumFractionDigits, expected) => {
		expect(formatNumber(input, maximumFractionDigits)).toBe(expected);
	});
});

describe("formatCompactNumber", () => {
	it("formats compact values", () => {
		expect(formatCompactNumber(1_234_567)).toBe("1.2M");
	});
});
