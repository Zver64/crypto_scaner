import { describe, expect, it } from "vitest";
import type { Candle, LiveCandleState } from "@/api/generated/models";
import {
	applyLiveCandleUpdate,
	mergeHistoryAndLiveCandles,
} from "@/features/instrument-analysis/live-candle-merge";

function candle(hour: number, close: number): Candle {
	return {
		open_time: `2026-01-01T${String(hour).padStart(2, "0")}:00:00Z`,
		close_time: `2026-01-01T${String(hour).padStart(2, "0")}:59:59Z`,
		open: close - 1,
		high: close + 1,
		low: close - 2,
		close,
		volume: 10,
		quote_asset_volume: 100,
		trade_count: 5,
	};
}

describe("mergeHistoryAndLiveCandles", () => {
	it("keeps a newer final live snapshot when a delayed HTTP response overlaps", () => {
		const history = [candle(10, 100), candle(11, 101)];
		const live: LiveCandleState[] = [
			{ candle: candle(11, 105), final: true },
			{ candle: candle(12, 106), final: false },
		];
		expect(
			mergeHistoryAndLiveCandles(history, live).map((item) => item.close),
		).toEqual([100, 105, 106]);
	});

	it("does not roll HTTP-confirmed closure back to a delayed forming event", () => {
		const history = [candle(11, 105)];
		const live = [{ candle: candle(11, 99), final: false }];
		expect(mergeHistoryAndLiveCandles(history, live)[0]?.close).toBe(105);
	});
});

describe("applyLiveCandleUpdate", () => {
	it("replaces whole snapshots, preserves finality, and bounds the recovery buffer", () => {
		let states: LiveCandleState[] = [];
		for (let hour = 1; hour <= 4; hour += 1) {
			states = applyLiveCandleUpdate(
				states,
				{ candle: candle(hour, hour), final: hour < 4 },
				3,
			);
		}
		states = applyLiveCandleUpdate(
			states,
			{ candle: candle(3, 999), final: false },
			3,
		);
		expect(states.map((state) => state.candle.close)).toEqual([2, 3, 4]);
	});
});
