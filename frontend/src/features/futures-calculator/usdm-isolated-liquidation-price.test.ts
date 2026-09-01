import { describe, expect, it } from "vitest";
import {
	calculateUsdmIsolatedLiquidationPrice,
	type UsdmIsolatedLiquidationPriceInput,
} from "./usdm-isolated-liquidation-price";

const validInput: UsdmIsolatedLiquidationPriceInput = {
	direction: "long",
	entryPrice: 100_000,
	leverage: 20,
	maintenanceMarginRatio: 0.004,
	quantity: 0.01,
};

describe("calculateUsdmIsolatedLiquidationPrice", () => {
	it("calculates a conservative isolated long liquidation estimate", () => {
		expect(calculateUsdmIsolatedLiquidationPrice(validInput)).toBeCloseTo(
			95_381.52610441767,
			12,
		);
	});

	it("calculates a conservative isolated short liquidation estimate", () => {
		expect(
			calculateUsdmIsolatedLiquidationPrice({
				...validInput,
				direction: "short",
			}),
		).toBeCloseTo(104_581.67330677291, 12);
	});

	it("uses zero maintenance margin without a separate special case", () => {
		expect(
			calculateUsdmIsolatedLiquidationPrice({
				direction: "long",
				entryPrice: 100,
				leverage: 10,
				maintenanceMarginRatio: 0,
				quantity: 1,
			}),
		).toBe(90);
	});

	it.each([
		[
			"entry price",
			"entryPrice",
			[0, -1, Number.NaN, Number.POSITIVE_INFINITY],
		],
		["leverage", "leverage", [0, -1, Number.NaN, Number.NEGATIVE_INFINITY]],
		["quantity", "quantity", [0, -1, Number.NaN, Number.POSITIVE_INFINITY]],
		[
			"maintenance-margin ratio",
			"maintenanceMarginRatio",
			[-0.01, 1, Number.NaN, Number.POSITIVE_INFINITY],
		],
	] as const)("rejects invalid %s", (_description, field, values) => {
		for (const value of values) {
			expect(() =>
				calculateUsdmIsolatedLiquidationPrice({
					...validInput,
					[field]: value,
				}),
			).toThrow();
		}
	});
});
