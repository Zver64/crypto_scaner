import Decimal from "decimal.js";
import { describe, expect, it } from "vitest";
import { calculateLinearAverageEntryPrice } from "@/utils/calculator/linear-average-entry-price";

describe("calculateLinearAverageEntryPrice", () => {
	it("returns the entry price for one fill", () => {
		expect(
			calculateLinearAverageEntryPrice([
				{ quantity: "0.01", price: "100000" },
			]).toString(),
		).toBe("100000");
	});

	it("returns an unrounded Decimal weighted average", () => {
		const average = calculateLinearAverageEntryPrice([
			{ quantity: "1", price: "100000" },
			{ quantity: new Decimal("2"), price: "90000" },
		]);

		expect(average).toBeInstanceOf(Decimal);
		expect(average.toSignificantDigits(16).toString()).toBe(
			"93333.33333333333",
		);
	});

	it("uses base-asset quantity as the weight", () => {
		const average = calculateLinearAverageEntryPrice([
			{ quantity: "0.01", price: "100000" },
			{ quantity: "0.02", price: "90000" },
		]);

		expect(average.toSignificantDigits(16).toString()).toBe(
			"93333.33333333333",
		);
	});

	it("preserves precision beyond JavaScript numbers", () => {
		const average = calculateLinearAverageEntryPrice([
			{ quantity: "0.00000000000000000001", price: "100000000000000000001" },
			{ quantity: "0.00000000000000000002", price: "90000000000000000001" },
		]);

		expect(average.toFixed(20)).toBe(
			"93333333333333333334.33333333333333333333",
		);
	});

	it("is independent of fill ordering", () => {
		const fills = [
			{ quantity: "1e16", price: "1" },
			{ quantity: "1", price: "2" },
			{ quantity: "1", price: "3" },
		];

		expect(
			calculateLinearAverageEntryPrice([...fills].reverse()).eq(
				calculateLinearAverageEntryPrice(fills),
			),
		).toBe(true);
	});

	it.each([
		["empty fills", []],
		["zero quantity", [{ quantity: 0, price: 100_000 }]],
		["negative quantity", [{ quantity: -1, price: 100_000 }]],
		["zero price", [{ quantity: 1, price: 0 }]],
		["negative price", [{ quantity: 1, price: -100_000 }]],
		["non-finite quantity", [{ quantity: Number.NaN, price: 100_000 }]],
		["non-finite price", [{ quantity: 1, price: Number.POSITIVE_INFINITY }]],
	] as const)("rejects %s", (_description, fills) => {
		expect(() => calculateLinearAverageEntryPrice(fills)).toThrow(RangeError);
	});
});
