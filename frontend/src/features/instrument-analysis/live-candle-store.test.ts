import { expect, it } from "vitest";
import type { Candle, LiveCandleServerMessage } from "@/api/generated/models";
import {
	applyServerMessage,
	createLiveStore,
} from "@/features/instrument-analysis/live-candle-store";

function candle(openTime: string): Candle {
	return {
		open_time: openTime,
		close_time: "2026-01-02T00:00:00Z",
		open: 100,
		high: 110,
		low: 90,
		close: 105,
		volume: 10,
		quote_asset_volume: 1000,
		trade_count: 5,
	};
}
function snapshot(
	interval: "1h" | "1d",
	time: string,
	version: number,
	value: number,
): LiveCandleServerMessage {
	return {
		type: "snapshot",
		symbol: "BTC",
		interval,
		version,
		chart: {
			symbol: "BTC",
			interval,
			candles: [candle(time)],
			indicators: [
				{
					type: "rsi",
					parameters: { period: 14 },
					series: [{ name: "rsi", points: [{ time, value }] }],
				},
			],
			has_more: false,
			next_before: null,
		},
	};
}
it("keeps the visible interval snapshot stable when a hidden interval updates", () => {
	const store = createLiveStore("BTC");
	const hourly = store.getSnapshot("1h");
	store.message({
		type: "status",
		symbol: "BTC",
		interval: "1d",
		freshness: "fresh",
	});
	expect(store.getSnapshot("1h")).toBe(hourly);
	expect(store.getSnapshot("1d").freshness).toBe("fresh");
});
it("retains isolated calculated snapshots and rejects late versions", () => {
	let states = applyServerMessage(
		{},
		snapshot("1h", "2026-01-01T23:00:00Z", 2, 45),
		"BTC",
		"connected",
	);
	states = applyServerMessage(
		states,
		snapshot("1d", "2026-01-01T00:00:00Z", 1, 55),
		"BTC",
		"connected",
	);
	const stable = states;
	states = applyServerMessage(
		states,
		snapshot("1h", "2026-01-01T23:00:00Z", 1, 99),
		"BTC",
		"connected",
	);
	expect(states).toBe(stable);
	states = applyServerMessage(
		states,
		snapshot("1d", "2026-01-01T00:00:00Z", 2, 60),
		"BTC",
		"connected",
	);
	expect(
		states["BTC:1h"]?.chart?.indicators[0]?.series[0]?.points[0]?.value,
	).toBe(45);
	expect(
		states["BTC:1d"]?.chart?.indicators[0]?.series[0]?.points[0]?.value,
	).toBe(60);
});
