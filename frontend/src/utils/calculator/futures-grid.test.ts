import { describe, expect, it } from "vitest";
import {
	calculateFuturesGrid,
	type FuturesGridInput,
} from "@/utils/calculator/futures-grid";

const longFixture: FuturesGridInput = {
	currentPrice: 115,
	direction: "long",
	gridCount: "2",
	gridType: "geometric",
	investment: "220",
	leverage: 1,
	lowerPrice: "100",
	upperPrice: "121",
};

describe("calculateFuturesGrid", () => {
	it("fills every long order without leverage and is never liquidated", () => {
		const result = calculateFuturesGrid(longFixture);

		expect(result.allocationPerOrder.toString()).toBe("110");
		expect(result.filledOrderCount).toBe(2);
		expect(result.averageEntryPrice.toFixed(8)).toBe("104.76190476");
		expect(result.liquidationPrice).toBeNull();
	});

	it("fills orders beyond the current price at market", () => {
		const result = calculateFuturesGrid({ ...longFixture, currentPrice: 105 });

		// 110 USDT at 105, then 110 USDT at 100.
		expect(result.averageEntryPrice.toFixed(8)).toBe("102.43902439");
	});

	it("fills every order at market when the current price is beyond the range", () => {
		const result = calculateFuturesGrid({ ...longFixture, currentPrice: 90 });

		expect(result.averageEntryPrice.toFixed(8)).toBe("90.00000000");
	});

	it("liquidates a leveraged short above the grid once every order fills", () => {
		const result = calculateFuturesGrid({
			...longFixture,
			currentPrice: 105,
			direction: "short",
			leverage: 3,
		});

		// 330 USDT at 110 and 330 USDT at 121, with 220 USDT of margin.
		expect(result.allocationPerOrder.toString()).toBe("330");
		expect(result.filledOrderCount).toBe(2);
		expect(result.averageEntryPrice.toFixed(8)).toBe("115.23809524");
		expect(result.liquidationPrice?.toFixed(8)).toBe("152.12949866");
	});

	it("liquidates before the price reaches the remaining orders", () => {
		const result = calculateFuturesGrid({
			currentPrice: 100,
			direction: "long",
			gridCount: "2",
			gridType: "arithmetic",
			investment: "100",
			leverage: 3,
			lowerPrice: "10",
			upperPrice: "100",
		});

		// The 55 order fills; LP = (150 − 100) / (150 / 55 × 0.99) lies
		// above the 10 order, so it never fills.
		expect(result.filledOrderCount).toBe(1);
		expect(result.averageEntryPrice.toFixed(8)).toBe("55.00000000");
		expect(result.liquidationPrice?.toFixed(8)).toBe("18.51851852");
	});

	it("charges the maker fee on both legs of a trade", () => {
		const long = calculateFuturesGrid(longFixture);
		const short = calculateFuturesGrid({ ...longFixture, direction: "short" });

		// Long: 10% gross − 0.02% × (1 + 1.1).
		expect(long.tradeMinimum.grossProfitPercent.toString()).toBe("10");
		expect(long.tradeMinimum.profitPercent.toFixed(6)).toBe("9.958000");
		expect(long.tradeMinimum.profit.toFixed(6)).toBe("10.953800");
		// Short: 1/11 gross − 0.02% × (1 + 1/1.1).
		expect(short.tradeMinimum.profitPercent.toFixed(6)).toBe("9.052727");
		expect(long.stepPercentMinimum.toString()).toBe("10");
		expect(long.stepPercentMaximum.toString()).toBe("10");
	});

	it("reports the lowest and highest arithmetic trades", () => {
		const result = calculateFuturesGrid({
			...longFixture,
			gridType: "arithmetic",
			lowerPrice: "100",
			upperPrice: "120",
		});

		expect(result.stepPercentMaximum.toString()).toBe("10");
		expect(result.stepPercentMinimum.toFixed(6)).toBe("9.090909");
		expect(result.tradeMaximum.profit.gt(result.tradeMinimum.profit)).toBe(
			true,
		);
	});

	it.each([
		{ ...longFixture, leverage: 0 },
		{ ...longFixture, leverage: 4 },
		{ ...longFixture, currentPrice: 0 },
		{ ...longFixture, currentPrice: Number.NaN },
		{ ...longFixture, upperPrice: "100" },
		{ ...longFixture, gridCount: "0" },
		{ ...longFixture, investment: "-1" },
	])("rejects invalid input %#", (input) => {
		expect(() => calculateFuturesGrid(input)).toThrow(RangeError);
	});
});
