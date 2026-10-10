import { describe, expect, it } from "vitest";
import { formatCompactNumber, formatNumber } from "@/utils/number-format";

describe("formatNumber", () => {
	it.each([
		["zero", "0", "0"],
		["small values", "0.0000000123456789", "0.00000001235"],
		["ordinary values", "0.2658481", "0.2658"],
		["large values", "12345678.9", "12,345,679"],
		["whole numbers", "5785", "5,785"],
		["large fractional values", "5785.4", "5,785"],
		["mid-sized values", "158.73", "158.7"],
		["values above one", "1.23456", "1.235"],
		["negative nonzero values", "-0.0000000123456789", "-0.00000001235"],
	])("formats %s keeping the integer part and adapting the fraction", (_, input, expected) => {
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
		[0.000000001, 2, "0"],
	])("formats %s with fraction limit %s without trailing zeros", (input, maximumFractionDigits, expected) => {
		expect(formatNumber(input, maximumFractionDigits)).toBe(expected);
	});

	it.each([
		[0, "0.00"],
		[1.2345, "1.23"],
		[123.456, "123.46"],
		[1234.567, "1234.57"],
	])("preserves fixed fractional precision for %s", (input, expected) => {
		expect(formatNumber(input, 2, "halfExpand", false, 2)).toBe(expected);
	});

	it("rounds in the requested direction", () => {
		expect(formatNumber("1597.9", undefined, "floor")).toBe("1,597");
		expect(formatNumber("1.09999", undefined, "floor")).toBe("1.099");
	});

	it("leaves out group separators on request", () => {
		expect(formatNumber("12345678.9", undefined, "floor", false)).toBe(
			"12345678",
		);
		expect(formatNumber("12345678.9")).toBe("12,345,679");
	});
});

describe("formatCompactNumber", () => {
	it("formats compact values", () => {
		expect(formatCompactNumber(1_234_567)).toBe("1.2M");
	});
});
