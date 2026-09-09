import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { describe, expect, it } from "vitest";
import {
	availableCandles,
	chartPriceResolution,
	createCandlestickData,
	formatOhlc,
	formatPrice,
	formatUtcTimestamp,
} from "@/features/instrument-analysis/price-history-chart/utils";

const from = "2026-08-26T23:00:00Z";

// Exercise the installed calculator rather than a mock or a copied validator.
// These internals are not exported; isolate them from the DOM-dependent chart.
const chartSource = readFileSync(
	new URL(
		"lightweight-charts.development.mjs",
		import.meta.resolve("lightweight-charts"),
	),
	"utf8",
);
const decimalCheck = chartSource.match(
	/^function isBaseDecimal\(value\) \{[\s\S]*?^\}/m,
)?.[0];
const tickCalculator = chartSource.match(
	/^class PriceTickSpanCalculator \{[\s\S]*?^\}/m,
)?.[0];
if (!decimalCheck || !tickCalculator) {
	throw new Error("Cannot locate installed price tick calculator");
}
const acceptsBase = runInNewContext(
	`${decimalCheck}\n${tickCalculator}\n(base) => new PriceTickSpanCalculator(base, [2, 2.5, 2])`,
) as (base: number) => unknown;

function candle(slot: number, close: number) {
	return {
		close,
		high: close + 1,
		low: close - 1,
		open: close - 0.5,
		open_time: new Date(Date.parse(from) + slot * 3_600_000).toISOString(),
	};
}

describe("candlestick chart presentation", () => {
	it("preserves missing hourly slots as TradingView whitespace data", () => {
		expect(
			createCandlestickData([candle(0, 10), null, candle(2, 12)], from),
		).toEqual([
			{ close: 10, high: 11, low: 9, open: 9.5, time: 1_787_785_200 },
			{ time: 1_787_788_800 },
			{ close: 12, high: 13, low: 11, open: 11.5, time: 1_787_792_400 },
		]);
	});

	it("returns only available candles without reordering", () => {
		const first = candle(2, 2);
		const second = candle(9, 9);
		expect(availableCandles([null, first, null, second])).toEqual([
			first,
			second,
		]);
	});

	it("uses sufficient chart precision for sub-cent instruments", () => {
		const data = createCandlestickData([candle(0, 0.00001234)], from);
		expect(chartPriceResolution(data)).toEqual({ base: 1e12, minMove: 1e-12 });
		expect(chartPriceResolution([{ time: 1 as never }])).toEqual({
			base: 100,
			minMove: 0.01,
		});
	});

	it("avoids reciprocal rounding for a 1e-11 instrument", () => {
		const price = 1e-11;
		const resolution = chartPriceResolution([
			{ open: price, high: price, low: price, close: price, time: 0 as never },
		]);
		expect(resolution).toEqual({ base: 1e18, minMove: 1e-18 });
		expect(() => acceptsBase(1 / resolution.minMove)).toThrow(
			"unexpected base",
		);
		expect(() => acceptsBase(resolution.base)).not.toThrow();
	});

	it.each([
		Number.MIN_VALUE,
		1e-320,
		1e-308,
		1e-300,
		1e300,
		Number.MAX_VALUE,
	])("keeps tick resolution valid for the extreme price %s", (price) => {
		const { base, minMove } = chartPriceResolution([
			{ open: price, high: price, low: price, close: price, time: 0 as never },
		]);
		expect(Number.isFinite(base)).toBe(true);
		expect(Number.isFinite(minMove)).toBe(true);
		expect(base).toBeGreaterThan(0);
		expect(minMove).toBeGreaterThan(0);
		expect(() => acceptsBase(base)).not.toThrow();
	});

	it("produces accepted decimal bases throughout the bounded exponent range", () => {
		for (let exponent = 2; exponent <= 308; exponent++) {
			const price = 10 ** (7 - exponent);
			const { base } = chartPriceResolution([
				{
					open: price,
					high: price,
					low: price,
					close: price,
					time: 0 as never,
				},
			]);
			expect(() => acceptsBase(base)).not.toThrow();
		}
	});

	it("formats UTC time and OHLC with useful price precision", () => {
		expect(formatUtcTimestamp("2026-08-26T23:00:00Z")).toBe(
			"Aug 26, 23:00 UTC",
		);
		expect(formatPrice(0.0000123456789)).toBe("0.0000123");
		expect(
			formatOhlc({ open: 1, high: 2, low: 0.5, close: 1.5, time: 0 as never }),
		).toBe("O 1  H 2  L 0.5  C 1.5");
	});
});
