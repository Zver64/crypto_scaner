import { describe, expect, it } from "vitest";
import { calculateInverseAverageEntryPrice } from "./inverse-average-entry-price";
import type { InverseAverageEntryPriceFill } from "./types";

const specifiedCoinmFills = [
	{ contractCount: 100, contractSize: 100, price: 25_000 },
	{ contractCount: 200, contractSize: 100, price: 20_000 },
] as const;

describe("calculateInverseAverageEntryPrice", () => {
	it("calculates the unrounded average from one COIN-M fill", () => {
		expect(
			calculateInverseAverageEntryPrice([
				{ contractCount: 100, contractSize: 100, price: 25_000 },
			]),
		).toBe(25_000);
	});

	it("calculates the contract-notional-weighted harmonic average", () => {
		expect(
			calculateInverseAverageEntryPrice([
				{ contractCount: 100, contractSize: 100, price: 100_000 },
				{ contractCount: 200, contractSize: 100, price: 90_000 },
			]),
		).toBeCloseTo(93_103.44827586206, 10);
	});

	it("calculates the specified COIN-M fixture regardless of fill order", () => {
		expect(calculateInverseAverageEntryPrice(specifiedCoinmFills)).toBe(
			21_428.571428571428,
		);
		expect(
			calculateInverseAverageEntryPrice([...specifiedCoinmFills].reverse()),
		).toBe(21_428.571428571428);
	});

	it("does not change when all contract counts are scaled equally", () => {
		expect(
			calculateInverseAverageEntryPrice(
				specifiedCoinmFills.map((fill) => ({
					...fill,
					contractCount: fill.contractCount * 10,
				})),
			),
		).toBe(calculateInverseAverageEntryPrice(specifiedCoinmFills));
	});

	it("uses inverse contract economics for base-asset entry notional", () => {
		const average = calculateInverseAverageEntryPrice([
			{ contractCount: 300, contractSize: 100, price: 25_000 },
		]);

		expect((300 * 100) / average).toBe(1.2);
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
		[
			"non-finite contract count",
			[{ contractCount: Number.NaN, contractSize: 100, price: 100_000 }],
		],
		[
			"non-finite contract size",
			[
				{
					contractCount: 1,
					contractSize: Number.POSITIVE_INFINITY,
					price: 100_000,
				},
			],
		],
		[
			"negative price",
			[{ contractCount: 1, contractSize: 100, price: -100_000 }],
		],
	] as const)("rejects %s", (_description, fills) => {
		expect(() => calculateInverseAverageEntryPrice(fills)).toThrow(RangeError);
	});

	it("rejects fills with opposing position directions", () => {
		const fills = [
			{
				contractCount: 100,
				contractSize: 100,
				price: 25_000,
				direction: "long",
			},
			{
				contractCount: 200,
				contractSize: 100,
				price: 20_000,
				direction: "short",
			},
		] satisfies readonly InverseAverageEntryPriceFill[];

		expect(() => calculateInverseAverageEntryPrice(fills)).toThrow(RangeError);
	});
});
