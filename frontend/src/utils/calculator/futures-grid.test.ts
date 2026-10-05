import { describe, expect, it } from "vitest";
import {
	calculateFuturesGrid,
	type FuturesGridInput,
} from "@/utils/calculator/futures-grid";

const longFixture: FuturesGridInput = {
	contract: "linear",
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
			contract: "linear",
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

describe("calculateFuturesGrid with inverse contracts", () => {
	const inverseFixture: FuturesGridInput = {
		...longFixture,
		contract: "inverse",
		currentPrice: 100,
		direction: "short",
		investment: "2",
	};

	it("never liquidates an unleveraged short, which hedges its margin", () => {
		const result = calculateFuturesGrid(inverseFixture);

		// 2 coins × 100 USD split into two 100 USD orders at 110 and 121.
		expect(result.allocationPerOrder.toString()).toBe("100");
		expect(result.filledOrderCount).toBe(2);
		expect(result.averageEntryPrice.toFixed(8)).toBe("115.23809524");
		expect(result.liquidationPrice).toBeNull();
	});

	it("liquidates an unleveraged long at about half its entry", () => {
		const result = calculateFuturesGrid({
			...inverseFixture,
			direction: "long",
			gridCount: "1",
		});

		// LP = N(1 + MMR) / (M + K) = 200 × 1.01 / (2 + 2).
		expect(result.liquidationPrice?.toString()).toBe("50.5");
	});

	it("liquidates a leveraged short above the grid", () => {
		const result = calculateFuturesGrid({
			...inverseFixture,
			currentPrice: 105,
			leverage: 3,
		});

		// LP = N(1 − MMR) / (K − M) with 315 USD orders at 110 and 121.
		expect(result.allocationPerOrder.toString()).toBe("315");
		expect(result.liquidationPrice?.toFixed(8)).toBe("179.89916567");
	});

	it("liquidates before the price reaches the remaining orders", () => {
		const result = calculateFuturesGrid({
			...inverseFixture,
			direction: "long",
			gridType: "arithmetic",
			investment: "1",
			leverage: 3,
			lowerPrice: "10",
			upperPrice: "100",
		});

		// The 55 order fills; LP = 150 × 1.01 / (1 + 150 / 55) ≈ 40.6 lies
		// above the 10 order, so it never fills.
		expect(result.filledOrderCount).toBe(1);
		expect(result.liquidationPrice?.toFixed(8)).toBe("40.64634146");
	});

	it("measures trade profit in the coin", () => {
		const long = calculateFuturesGrid({
			...inverseFixture,
			direction: "long",
			gridCount: "2",
			upperPrice: "121",
		});
		const short = calculateFuturesGrid(inverseFixture);

		// 100 USD between 100 and 110 gains 100 × (1/100 − 1/110) coin gross,
		// and the long and short trades net the same coin amount.
		expect(long.tradeMaximum.grossProfit.toFixed(8)).toBe("0.09090909");
		expect(long.tradeMaximum.profit.toFixed(8)).toBe("0.09052727");
		expect(short.tradeMaximum.profit.toFixed(8)).toBe("0.09052727");
		// Geometric steps share one percent, but not one coin amount.
		expect(long.tradeMinimum.profitPercent.toFixed(8)).toBe(
			long.tradeMaximum.profitPercent.toFixed(8),
		);
		expect(long.tradeMinimum.profit.lt(long.tradeMaximum.profit)).toBe(true);
	});
});
