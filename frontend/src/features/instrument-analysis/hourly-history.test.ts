import { describe, expect, it } from "vitest";
import type { PriceCandle } from "@/features/instrument-analysis/candle-page";
import { currentSevenDayHourlyCloses } from "@/features/instrument-analysis/hourly-history";

const hour = 60 * 60 * 1_000;
const now = Date.parse("2026-09-15T16:30:00Z");
const lastClosedOpen = Date.parse("2026-09-15T15:00:00Z");

function candle(open: number, close: number): PriceCandle {
	return {
		close,
		high: close,
		low: close,
		open: close,
		open_time: new Date(open).toISOString(),
	};
}

describe("currentSevenDayHourlyCloses", () => {
	it("uses the current closed seven-day UTC window despite internal gaps", () => {
		expect(
			currentSevenDayHourlyCloses(
				[
					candle(lastClosedOpen - 169 * hour, 1),
					candle(lastClosedOpen - 168 * hour, 2),
					candle(lastClosedOpen - 12 * hour, 3),
					candle(lastClosedOpen, 4),
				],
				now,
			),
		).toEqual([2, 3, 4]);
	});

	it("rejects stale and forming-hour coverage", () => {
		expect(
			currentSevenDayHourlyCloses(
				[
					candle(lastClosedOpen - 200 * hour, 1),
					candle(lastClosedOpen + hour, 2),
				],
				now,
			),
		).toEqual([]);
	});
});
