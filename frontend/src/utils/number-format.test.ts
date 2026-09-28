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
});

describe("formatCompactNumber", () => {
	it("formats compact values", () => {
		expect(formatCompactNumber(1_234_567)).toBe("1.2M");
	});
});
