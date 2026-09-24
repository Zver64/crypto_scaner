import { describe, expect, it } from "vitest";
import {
	createCandlestickData,
	createIndicatorData,
	formatOhlc,
	formatPrice,
} from "@/components/price-history-chart/utils";

const from = "2026-08-26T23:00:00Z";
const nextHour = (value: string) =>
	new Date(Date.parse(value) + 3_600_000).toISOString();

function candle(slot: number, close: number) {
	return {
		close,
		high: close + 1,
		low: close - 1,
		open: close - 0.5,
		open_time: new Date(Date.parse(from) + slot * 3_600_000).toISOString(),
	};
}

describe("price history chart data", () => {
	it("preserves missing candle slots without changing observed candle times", () => {
		expect(
			createCandlestickData([candle(0, 10), candle(2, 12)], "1h", nextHour),
		).toEqual([
			{ close: 10, high: 11, low: 9, open: 9.5, time: 1_787_785_200 },
			{ time: 1_787_788_800 },
			{ close: 12, high: 13, low: 11, open: 11.5, time: 1_787_792_400 },
		]);
	});

	it("aligns an indicator on the existing grid, including gaps and warm-up slots", () => {
		const candles = [candle(0, 10), candle(2, 12), candle(3, 13)];
		let advances = 0;
		const data = createCandlestickData(candles, "1h", (time) => {
			advances++;
			return nextHour(time);
		});
		const afterCandles = advances;
		expect(
			createIndicatorData(data, [
				{ time: candles[1].open_time, value: 45 },
				{ time: candles[2].open_time, value: 55 },
			]),
		).toEqual([
			{ time: 1_787_785_200 },
			{ time: 1_787_788_800 },
			{ time: 1_787_792_400, value: 45 },
			{ time: 1_787_796_000, value: 55 },
		]);
		expect(advances).toBe(afterCandles);
	});

	it("does not insert duplicate slots for equivalent ISO timestamp formats", () => {
		const first = candle(0, 10);
		const second = candle(1, 11);
		first.open_time = first.open_time.replace(".000Z", "Z");
		second.open_time = second.open_time.replace(".000Z", "Z");
		expect(
			createCandlestickData([first, second], "1h", nextHour).map(
				({ time }) => time,
			),
		).toEqual([1_787_785_200, 1_787_788_800]);
	});

	it("formats OHLC with useful price precision", () => {
		expect(formatPrice(0.0000123456789)).toBe("0.0000123");
		expect(
			formatOhlc({ open: 1, high: 2, low: 0.5, close: 1.5, time: 0 }),
		).toBe("O 1  H 2  L 0.5  C 1.5");
	});
});
