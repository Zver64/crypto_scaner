import Decimal from "decimal.js";
import { describe, expect, it } from "vitest";
import type { UsdmIsolatedLiquidationPriceInput } from "@/utils/calculator/types";
import { calculateUsdmIsolatedLiquidationPrice } from "@/utils/calculator/usdm-isolated-liquidation-price";

const validInput: UsdmIsolatedLiquidationPriceInput = {
	direction: "long",
	entryPrice: "100000",
	leverage: "20",
	maintenanceMarginRatio: "0.004",
	quantity: "0.01",
};

describe("calculateUsdmIsolatedLiquidationPrice", () => {
	it("returns a conservative isolated long estimate as Decimal", () => {
		const result = calculateUsdmIsolatedLiquidationPrice(validInput);

		expect(result).toBeInstanceOf(Decimal);
		expect(result.toSignificantDigits(16).toString()).toBe("95381.52610441767");
	});

	it("calculates a conservative isolated short liquidation estimate", () => {
		expect(
			calculateUsdmIsolatedLiquidationPrice({
				...validInput,
				direction: "short",
			})
				.toSignificantDigits(16)
				.toString(),
		).toBe("104581.6733067729");
	});

	it("uses zero maintenance margin and preserves precise Decimal inputs", () => {
		const result = calculateUsdmIsolatedLiquidationPrice({
			direction: "long",
			entryPrice: new Decimal("100.00000000000000000001"),
			leverage: "10",
			maintenanceMarginRatio: "0",
			quantity: "1",
		});

		expect(result.toString()).toBe("90.000000000000000000009");
	});

	it("preserves an exact zero from long leverage of one", () => {
		const result = calculateUsdmIsolatedLiquidationPrice({
			...validInput,
			leverage: "1",
		});

		expect(result.isZero()).toBe(true);
	});

	it("preserves a negative result allowed by sub-one long leverage", () => {
		const result = calculateUsdmIsolatedLiquidationPrice({
			...validInput,
			leverage: "0.5",
		});

		expect(result.isNegative()).toBe(true);
	});

	it("rejects a nonfinite derived result", () => {
		expect(() =>
			calculateUsdmIsolatedLiquidationPrice({
				...validInput,
				direction: "short",
				entryPrice: "9e9000000000000000",
				leverage: "1",
				maintenanceMarginRatio: "0",
			}),
		).toThrow(RangeError);
	});

	it("rejects arithmetic underflow without rejecting exact cancellation", () => {
		expect(() =>
			calculateUsdmIsolatedLiquidationPrice({
				...validInput,
				entryPrice: "1e-9000000000000000",
				leverage: `1.${"0".repeat(58)}1`,
				maintenanceMarginRatio: "0",
			}),
		).toThrow(RangeError);
	});

	it("rejects a leverage adjustment rounded to zero", () => {
		expect(() =>
			calculateUsdmIsolatedLiquidationPrice({
				...validInput,
				leverage: `1.${"0".repeat(60)}1`,
			}),
		).toThrow(RangeError);
	});

	it.each([
		["entry price", "entryPrice", [0, "-1", "NaN", "Infinity"]],
		["leverage", "leverage", [0, "-1", "NaN", "-Infinity"]],
		["quantity", "quantity", [0, "-1", "NaN", "Infinity"]],
		[
			"maintenance-margin ratio",
			"maintenanceMarginRatio",
			["-0.01", "1", "NaN", "Infinity"],
		],
	] as const)("rejects invalid %s", (_description, field, values) => {
		for (const value of values) {
			expect(() =>
				calculateUsdmIsolatedLiquidationPrice({
					...validInput,
					[field]: value,
				}),
			).toThrow(RangeError);
		}
	});
});
