import { afterEach, describe, expect, it, vi } from "vitest";
import { formatCompactNumber, formatNumber } from "@/utils/number-format";

describe("formatNumber", () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it.each([
		["zero", "0", "0"],
		["small values", "0.0000000123456789", "0.0000000123"],
		["ordinary values", "0.2658481", "0.266"],
		["large values", "12345678.9", "12,300,000"],
		["negative nonzero values", "-0.0000000123456789", "-0.0000000123"],
	])("formats %s with adaptive significant digits", (_, input, expected) => {
		expect(formatNumber(input)).toBe(expected);
	});

	it("does not turn a large fixed numeric string into infinity", () => {
		const value = `1${"0".repeat(900)}`;

		expect(formatNumber(value)).toBe(value);
	});

	it("falls back to the source when Intl rounds a nonzero value to zero", () => {
		vi.spyOn(Intl.NumberFormat.prototype, "formatToParts").mockReturnValue([
			{ type: "integer", value: "0" },
		]);

		expect(formatNumber("0.0000001")).toBe("0.0000001");
	});
});

describe("formatCompactNumber", () => {
	it("formats compact values", () => {
		expect(formatCompactNumber(1_234_567)).toBe("1.2M");
	});
});
