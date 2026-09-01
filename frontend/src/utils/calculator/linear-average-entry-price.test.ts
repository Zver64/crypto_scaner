import { describe, expect, it } from "vitest";
import { calculateLinearAverageEntryPrice } from "./linear-average-entry-price";

describe("calculateLinearAverageEntryPrice", () => {
	it("calculates the unrounded weighted average from one fill", () => {
		expect(
			calculateLinearAverageEntryPrice([{ quantity: 1, price: 100_000 }]),
		).toBe(100_000);
	});

	it("calculates the unrounded weighted average from multiple fills", () => {
		expect(
			calculateLinearAverageEntryPrice([
				{ quantity: 1, price: 100_000 },
				{ quantity: 2, price: 90_000 },
			]),
		).toBe(93_333.33333333333);
	});

	it("uses base-asset quantity as the weight", () => {
		expect(
			calculateLinearAverageEntryPrice([
				{ quantity: 0.01, price: 100_000 },
				{ quantity: 0.02, price: 90_000 },
			]),
		).toBeCloseTo(93_333.33333333333, 10);
	});

	it("is independent of fill ordering", () => {
		const fills = [
			{ quantity: 1e16, price: 1 },
			{ quantity: 1, price: 2 },
			{ quantity: 1, price: 3 },
		];

		expect(calculateLinearAverageEntryPrice([...fills].reverse())).toBe(
			calculateLinearAverageEntryPrice(fills),
		);
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
