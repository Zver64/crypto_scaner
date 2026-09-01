import { describe, expect, it } from "vitest";
import { calculateInverseAverageEntryPrice } from "./inverse-average-entry-price";

describe("calculateInverseAverageEntryPrice", () => {
	it("calculates the unrounded average from one COIN-M fill", () => {
		expect(
			calculateInverseAverageEntryPrice([
				{ contractCount: 100, contractSize: 100, price: 100_000 },
			]),
		).toBe(100_000);
	});

	it("calculates the contract-notional-weighted harmonic average", () => {
		expect(
			calculateInverseAverageEntryPrice([
				{ contractCount: 100, contractSize: 100, price: 100_000 },
				{ contractCount: 200, contractSize: 100, price: 90_000 },
			]),
		).toBeCloseTo(93_103.44827586206, 10);
	});

	it("is independent of fill ordering", () => {
		const fills = [
			{ contractCount: 1e16, contractSize: 1, price: 1 },
			{ contractCount: 1, contractSize: 1, price: 2 },
			{ contractCount: 1, contractSize: 1, price: 3 },
		];

		expect(calculateInverseAverageEntryPrice([...fills].reverse())).toBe(
			calculateInverseAverageEntryPrice(fills),
		);
	});

	it.each([
		["empty fills", []],
		[
			"zero contract count",
			[{ contractCount: 0, contractSize: 100, price: 100_000 }],
		],
		[
			"negative contract count",
			[{ contractCount: -1, contractSize: 100, price: 100_000 }],
		],
		[
			"zero contract size",
			[{ contractCount: 1, contractSize: 0, price: 100_000 }],
		],
		[
			"negative contract size",
			[{ contractCount: 1, contractSize: -100, price: 100_000 }],
		],
		["zero price", [{ contractCount: 1, contractSize: 100, price: 0 }]],
		[
			"non-finite price",
			[
				{
					contractCount: 1,
					contractSize: 100,
					price: Number.POSITIVE_INFINITY,
				},
			],
		],
	] as const)("rejects %s", (_description, fills) => {
		expect(() => calculateInverseAverageEntryPrice(fills)).toThrow(RangeError);
	});
});
