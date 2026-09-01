import { describe, expect, it } from "vitest";
import { calculateCoinmIsolatedLiquidationPrice } from "./coinm-isolated-liquidation-price";
import type { CoinmIsolatedLiquidationPriceOptions } from "./types";

const validLongOptions: CoinmIsolatedLiquidationPriceOptions = {
	direction: "long",
	contractCount: 100,
	contractSize: 100,
	entryPrice: 25_000,
	isolatedWalletBalance: 0.02,
	maintenanceMarginRatio: 0.004,
};

describe("calculateCoinmIsolatedLiquidationPrice", () => {
	it("calculates the level-one isolated long liquidation estimate", () => {
		expect(calculateCoinmIsolatedLiquidationPrice(validLongOptions)).toBe(
			23_904.761904761905,
		);
	});

	it("calculates the level-one isolated short liquidation estimate", () => {
		expect(
			calculateCoinmIsolatedLiquidationPrice({
				...validLongOptions,
				direction: "short",
			}),
		).toBe(26_210.526315789473);
	});

	it.each([
		[
			"contract count",
			"contractCount",
			[0, -1, Number.NaN, Number.POSITIVE_INFINITY],
		],
		[
			"contract size",
			"contractSize",
			[0, -1, Number.NaN, Number.NEGATIVE_INFINITY],
		],
		[
			"entry price",
			"entryPrice",
			[0, -1, Number.NaN, Number.POSITIVE_INFINITY],
		],
		[
			"isolated wallet balance",
			"isolatedWalletBalance",
			[0, -1, Number.NaN, Number.NEGATIVE_INFINITY],
		],
		[
			"maintenance-margin ratio",
			"maintenanceMarginRatio",
			[-0.01, 1, Number.NaN, Number.POSITIVE_INFINITY],
		],
	] as const)("rejects an invalid %s", (_description, field, values) => {
		for (const value of values) {
			expect(() =>
				calculateCoinmIsolatedLiquidationPrice({
					...validLongOptions,
					[field]: value,
				}),
			).toThrow(RangeError);
		}
	});
});
