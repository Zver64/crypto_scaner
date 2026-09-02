import { describe, expect, it } from "vitest";
import { calculateUsdmAverageEntryPrice } from "./usdm-average-entry-price";

describe("calculateUsdmAverageEntryPrice", () => {
	const binanceEntries = [
		{ price: "100000", quoteNotional: "1" },
		{ price: "90000", quoteNotional: "2" },
	];

	it("matches the Binance Open Price fixture before tick rounding", () => {
		const average = calculateUsdmAverageEntryPrice(binanceEntries);

		expect(average.toSignificantDigits(16).toString()).toBe(
			"93103.44827586207",
		);
		expect(average.toDecimalPlaces(2, 1).toString()).toBe("93103.44");
	});

	it("derives the expected base quantities without precision loss", () => {
		const average = calculateUsdmAverageEntryPrice(binanceEntries);

		expect(
			average
				.times("0.000032222222222222222")
				.toSignificantDigits(16)
				.toString(),
		).toBe("3");
	});

	it("returns the entry price for one entry", () => {
		expect(
			calculateUsdmAverageEntryPrice([
				{ price: 100_000, quoteNotional: 1 },
			]).toString(),
		).toBe("100000");
	});

	it("is independent of entry ordering", () => {
		expect(
			calculateUsdmAverageEntryPrice([...binanceEntries].reverse()),
		).toEqual(calculateUsdmAverageEntryPrice(binanceEntries));
	});

	it.each([
		["empty entries", []],
		["zero entry price", [{ price: 0, quoteNotional: 1 }]],
		["negative entry price", [{ price: -1, quoteNotional: 1 }]],
		["non-finite entry price", [{ price: Number.NaN, quoteNotional: 1 }]],
		["zero quote notional", [{ price: 100_000, quoteNotional: 0 }]],
		["negative quote notional", [{ price: 100_000, quoteNotional: -1 }]],
		[
			"non-finite quote notional",
			[
				{
					price: 100_000,
					quoteNotional: Number.POSITIVE_INFINITY,
				},
			],
		],
	] as const)("rejects %s", (_description, entries) => {
		expect(() => calculateUsdmAverageEntryPrice(entries as never)).toThrow(
			RangeError,
		);
	});
});
