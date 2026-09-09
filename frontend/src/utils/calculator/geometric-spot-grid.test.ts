import { describe, expect, it } from "vitest";
import { calculateGeometricSpotGrid } from "@/utils/calculator/geometric-spot-grid";

const positiveFixture = {
	lowerPrice: "100",
	upperPrice: "121",
	gridCount: "2",
	investment: "220",
};

describe("calculateGeometricSpotGrid", () => {
	it("uses equal percentage intervals, excludes the upper level, and applies both fees", () => {
		const result = calculateGeometricSpotGrid(positiveFixture);

		expect(result.gridCount).toBe(2);
		expect(result.levelCount).toBe(3);
		expect(result.stepRatio.toString()).toBe("1.1");
		expect(result.stepPercent.toString()).toBe("10");
		expect(result.allocationPerBuy.toString()).toBe("110");
		expect(result.totalNetQuantity.toString()).toBe("2.0979");
		expect(result.averageEntryPrice.toFixed(8)).toBe("104.86677153");
		expect(result.cycleProfit.toFixed(6)).toBe("10.758121");
		expect(result.cycleProfitPercent.toFixed(5)).toBe("9.78011");
	});

	it("keeps the same cycle return at every geometric interval", () => {
		const result = calculateGeometricSpotGrid({
			lowerPrice: "25",
			upperPrice: "400",
			gridCount: "4",
			investment: "1000",
		});

		expect(result.stepRatio.toString()).toBe("2");
		expect(result.stepPercent.toString()).toBe("100");
		expect(result.cycleProfitPercent.toFixed(4)).toBe("99.6002");
	});

	it("keeps a cycle visibly negative when fees exceed spacing", () => {
		const result = calculateGeometricSpotGrid({
			lowerPrice: "100",
			upperPrice: "100.1",
			gridCount: "1",
			investment: "100",
		});

		expect(result.cycleProfit.toFixed(7)).toBe("-0.1000999");
		expect(result.cycleProfitPercent.toFixed(7)).toBe("-0.1000999");
	});

	it("supports tiny valid prices with Decimal exponentiation", () => {
		const result = calculateGeometricSpotGrid({
			lowerPrice: "1e-900",
			upperPrice: "1.0001e-900",
			gridCount: "100",
			investment: "1e-20",
		});

		expect(result.stepRatio.gt(1)).toBe(true);
		expect(result.stepPercent.gt(0)).toBe(true);
		expect(result.averageEntryPrice.isFinite()).toBe(true);
	});

	it.each([
		{
			name: "net-quantity overflow",
			input: {
				lowerPrice: "1e-1000",
				upperPrice: "2e-1000",
				gridCount: "1",
				investment: "1e1000",
			},
		},
		{
			name: "net-quantity underflow",
			input: {
				lowerPrice: "1e1000",
				upperPrice: "2e1000",
				gridCount: "1",
				investment: "1e-1000",
			},
		},
		{
			name: "cycle-profit overflow",
			input: {
				lowerPrice: "1",
				upperPrice: "1e1000",
				gridCount: "1",
				investment: "1e1000",
			},
		},
	])("rejects derived $name instead of returning zero or Infinity", ({
		input,
	}) => {
		expect(() => calculateGeometricSpotGrid(input)).toThrow(RangeError);
	});

	it.each([
		{ ...positiveFixture, lowerPrice: "0" },
		{ ...positiveFixture, upperPrice: "100" },
		{ ...positiveFixture, investment: "-1" },
		{ ...positiveFixture, gridCount: "0" },
		{ ...positiveFixture, gridCount: "1001" },
	])("rejects invalid input %#", (input) => {
		expect(() => calculateGeometricSpotGrid(input)).toThrow(RangeError);
	});
});
