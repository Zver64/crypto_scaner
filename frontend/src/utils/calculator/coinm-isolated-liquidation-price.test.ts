import Decimal from "decimal.js";
import { describe, expect, it } from "vitest";
import { calculateCoinmIsolatedLiquidationPrice } from "@/utils/calculator/coinm-isolated-liquidation-price";
import type { CoinmIsolatedLiquidationPriceOptions } from "@/utils/calculator/types";

const validLongOptions: CoinmIsolatedLiquidationPriceOptions = {
	direction: "long",
	contractCount: 100,
	contractSize: "100",
	entryPrice: "25000",
	isolatedWalletBalance: "0.02",
	maintenanceMarginRatio: "0.004",
};

describe("calculateCoinmIsolatedLiquidationPrice", () => {
	it("returns the level-one isolated long estimate as Decimal", () => {
		const result = calculateCoinmIsolatedLiquidationPrice(validLongOptions);

		expect(result).toBeInstanceOf(Decimal);
		expect(result.toSignificantDigits(16).toString()).toBe("23904.7619047619");
	});

	it("calculates the level-one isolated short liquidation estimate", () => {
		expect(
			calculateCoinmIsolatedLiquidationPrice({
				...validLongOptions,
				direction: "short",
			})
				.toSignificantDigits(16)
				.toString(),
		).toBe("26210.52631578947");
	});

	it("preserves decimal input precision", () => {
		const result = calculateCoinmIsolatedLiquidationPrice({
			...validLongOptions,
			entryPrice: "25000.000000000000000001",
		});

		expect(result.decimalPlaces()).toBeGreaterThan(15);
	});

	it("preserves a negative short result when wallet balance exceeds entry value", () => {
		const result = calculateCoinmIsolatedLiquidationPrice({
			...validLongOptions,
			direction: "short",
			isolatedWalletBalance: "1",
		});

		expect(result.isNegative()).toBe(true);
	});

	it("rejects overflow in a required derived intermediate", () => {
		expect(() =>
			calculateCoinmIsolatedLiquidationPrice({
				...validLongOptions,
				contractCount: Number.MAX_VALUE,
				contractSize: "9e9000000000000000",
				entryPrice: "9e9000000000000000",
				isolatedWalletBalance: "1",
				maintenanceMarginRatio: "0",
			}),
		).toThrow(RangeError);
	});

	it("rejects underflow in a required derived intermediate", () => {
		expect(() =>
			calculateCoinmIsolatedLiquidationPrice({
				...validLongOptions,
				contractCount: Number.MIN_VALUE,
				contractSize: "1e-9000000000000000",
				entryPrice: "1",
			}),
		).toThrow(RangeError);
	});

	it.each([
		[
			"contract count",
			"contractCount",
			[0, -1, Number.NaN, Number.POSITIVE_INFINITY],
		],
		["contract size", "contractSize", [0, "-1", "NaN", "Infinity"]],
		["entry price", "entryPrice", [0, "-1", "NaN", "Infinity"]],
		[
			"isolated wallet balance",
			"isolatedWalletBalance",
			[0, "-1", "NaN", "Infinity"],
		],
		[
			"maintenance-margin ratio",
			"maintenanceMarginRatio",
			["-0.01", "1", "NaN", "Infinity"],
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
