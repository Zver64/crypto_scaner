import { describe, expect, it } from "vitest";
import { calculateArithmeticSpotGrid } from "@/utils/calculator/arithmetic-spot-grid";

const positiveFixture = {
	lowerPrice: "100",
	upperPrice: "120",
	gridCount: "2",
	investment: "220",
};

describe("calculateArithmeticSpotGrid", () => {
	it("uses N intervals, excludes the upper level from buys, and applies both fees", () => {
		const result = calculateArithmeticSpotGrid(positiveFixture);

		expect(result.gridCount).toBe(2);
		expect(result.levelCount).toBe(3);
		expect(result.allocationPerBuy.toString()).toBe("110");
		expect(result.stepPrice.toString()).toBe("10");
		expect(result.totalNetQuantity.toString()).toBe("2.0979");
		expect(result.averageEntryPrice.toFixed(8)).toBe("104.86677153");
		expect(result.stepPercentMinimum.toFixed(6)).toBe("9.090909");
		expect(result.stepPercentMaximum.toString()).toBe("10");
		expect(result.cycleProfitMaximum.toFixed(6)).toBe("10.758121");
		expect(result.cycleProfitMaximumPercent.toFixed(5)).toBe("9.78011");
		expect(result.cycleProfitMinimum.toFixed(5)).toBe("9.76012");
		expect(result.cycleProfitMinimumPercent.toFixed(8)).toBe("8.87283636");
	});

	it("keeps a cycle visibly negative when fees exceed spacing", () => {
		const result = calculateArithmeticSpotGrid({
			lowerPrice: "100",
			upperPrice: "100.1",
			gridCount: "1",
			investment: "100",
		});

		expect(result.averageEntryPrice.toFixed(7)).toBe("100.1001001");
		expect(result.cycleProfitMinimum.toFixed(7)).toBe("-0.1000999");
		expect(result.cycleProfitMinimumPercent.toFixed(7)).toBe("-0.1000999");
	});

	it("scales quantities and profit with investment but not prices or percentages", () => {
		const original = calculateArithmeticSpotGrid(positiveFixture);
		const doubled = calculateArithmeticSpotGrid({
			...positiveFixture,
			investment: "440",
		});

		expect(
			doubled.totalNetQuantity.eq(original.totalNetQuantity.times(2)),
		).toBe(true);
		expect(
			doubled.cycleProfitMinimum.eq(original.cycleProfitMinimum.times(2)),
		).toBe(true);
		expect(doubled.averageEntryPrice.eq(original.averageEntryPrice)).toBe(true);
		expect(
			doubled.cycleProfitMinimumPercent.eq(original.cycleProfitMinimumPercent),
		).toBe(true);
	});

	it("can produce a mixed-sign cycle range without hiding the loss", () => {
		const result = calculateArithmeticSpotGrid({
			lowerPrice: "100",
			upperPrice: "139.9",
			gridCount: "190",
			investment: "1900",
		});

		expect(result.cycleProfitMaximum.gt(0)).toBe(true);
		expect(result.cycleProfitMinimum.lt(0)).toBe(true);
	});

	it("supports tiny valid prices with Decimal arithmetic", () => {
		const result = calculateArithmeticSpotGrid({
			lowerPrice: "1e-900",
			upperPrice: "1.0001e-900",
			gridCount: "100",
			investment: "1e-20",
		});

		expect(result.stepPrice.gt(0)).toBe(true);
		expect(result.averageEntryPrice.isFinite()).toBe(true);
	});

	it("treats leading fractional zeros as non-significant", () => {
		const plain = calculateArithmeticSpotGrid({
			lowerPrice: `0.${"0".repeat(50)}1`,
			upperPrice: `0.${"0".repeat(50)}2`,
			gridCount: "1",
			investment: "1",
		});
		const exponent = calculateArithmeticSpotGrid({
			lowerPrice: "1e-51",
			upperPrice: "2e-51",
			gridCount: "1",
			investment: "1",
		});

		expect(plain.averageEntryPrice.eq(exponent.averageEntryPrice)).toBe(true);
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
		expect(() => calculateArithmeticSpotGrid(input)).toThrow(RangeError);
	});

	it.each([
		{ ...positiveFixture, lowerPrice: "" },
		{ ...positiveFixture, lowerPrice: " 100" },
		{ ...positiveFixture, lowerPrice: "NaN" },
		{ ...positiveFixture, lowerPrice: "Infinity" },
		{ ...positiveFixture, lowerPrice: "0" },
		{ ...positiveFixture, upperPrice: "100" },
		{ ...positiveFixture, upperPrice: "99" },
		{ ...positiveFixture, investment: "-1" },
		{ ...positiveFixture, gridCount: "1.5" },
		{ ...positiveFixture, gridCount: "0" },
		{ ...positiveFixture, gridCount: "1001" },
		{ ...positiveFixture, lowerPrice: "1e1001" },
		{ ...positiveFixture, lowerPrice: `1.${"1".repeat(51)}` },
	])("rejects invalid or unsupported input %#", (input) => {
		expect(() => calculateArithmeticSpotGrid(input)).toThrow(RangeError);
	});
});
