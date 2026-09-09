import Decimal from "decimal.js";
import { describe, expect, it } from "vitest";
import { calculateInverseAverageEntryPrice } from "@/utils/calculator/inverse-average-entry-price";
import type { InverseAverageEntryPriceFill } from "@/utils/calculator/types";

const specifiedCoinmFills = [
	{ contractCount: 100, contractSize: "100", price: "25000" },
	{ contractCount: 200, contractSize: "100", price: "20000" },
] as const;

describe("calculateInverseAverageEntryPrice", () => {
	it("returns the contract-notional-weighted harmonic average as Decimal", () => {
		const average = calculateInverseAverageEntryPrice(specifiedCoinmFills);

		expect(average).toBeInstanceOf(Decimal);
		expect(average.toSignificantDigits(16).toString()).toBe(
			"21428.57142857143",
		);
	});

	it("accepts Decimal values and preserves precision beyond numbers", () => {
		const average = calculateInverseAverageEntryPrice([
			{
				contractCount: 1,
				contractSize: new Decimal("100.00000000000000000001"),
				price: "25000.00000000000000000001",
			},
		]);

		expect(average.toString()).toBe("25000.00000000000000000001");
	});

	it("is independent of fill ordering and uniform contract-count scaling", () => {
		const reversed = calculateInverseAverageEntryPrice(
			[...specifiedCoinmFills].reverse(),
		);
		const scaled = calculateInverseAverageEntryPrice(
			specifiedCoinmFills.map((fill) => ({
				...fill,
				contractCount: fill.contractCount * 10,
			})),
		);
		const original = calculateInverseAverageEntryPrice(specifiedCoinmFills);

		expect(reversed.eq(original)).toBe(true);
		expect(scaled.eq(original)).toBe(true);
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
			"non-finite contract count",
			[{ contractCount: Number.NaN, contractSize: 100, price: 100_000 }],
		],
		[
			"zero contract size",
			[{ contractCount: 1, contractSize: 0, price: 100_000 }],
		],
		[
			"negative contract size",
			[{ contractCount: 1, contractSize: "-100", price: 100_000 }],
		],
		[
			"non-finite contract size",
			[{ contractCount: 1, contractSize: "Infinity", price: 100_000 }],
		],
		["zero price", [{ contractCount: 1, contractSize: 100, price: 0 }]],
		[
			"negative price",
			[{ contractCount: 1, contractSize: 100, price: "-100000" }],
		],
		[
			"non-finite price",
			[{ contractCount: 1, contractSize: 100, price: "Infinity" }],
		],
	] as const)("rejects %s", (_description, fills) => {
		expect(() => calculateInverseAverageEntryPrice(fills)).toThrow(RangeError);
	});

	it("rejects fills with opposing position directions", () => {
		const fills = [
			{ ...specifiedCoinmFills[0], direction: "long" },
			{ ...specifiedCoinmFills[1], direction: "short" },
		] satisfies readonly InverseAverageEntryPriceFill[];

		expect(() => calculateInverseAverageEntryPrice(fills)).toThrow(RangeError);
	});
});
