import { describe, expect, it } from "vitest";
import { normalizePriceTarget } from "@/features/price-alerts/target";

describe("normalizePriceTarget", () => {
	it.each([
		["1.0", "1"],
		[".5", "0.5"],
		["1e-8", "0.00000001"],
	])("normalizes %s", (input, expected) => {
		expect(normalizePriceTarget(input)).toEqual({ value: expected });
	});

	it.each([
		"",
		"0",
		"-1",
		"NaN",
		"0.0000000000000000001",
	])("rejects %s", (input) =>
		expect(normalizePriceTarget(input)).toHaveProperty("error"));

	it("enforces NUMERIC(38,18) integer precision", () => {
		expect(normalizePriceTarget("123456789012345678901")).toHaveProperty(
			"error",
		);
	});
});
