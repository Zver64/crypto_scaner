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

it("retains independent live snapshots when switching chart intervals", () => {
	const hourly: LiveCandleServerMessage = {
		type: "snapshot",
		symbol: "BTC",
		interval: "1h",
		freshness: "fresh",
		candles: [{ candle: candle("2026-01-01T23:00:00Z"), final: false }],
	};
	const daily: LiveCandleServerMessage = {
		type: "snapshot",
		symbol: "BTC",
		interval: "1d",
		freshness: "fresh",
		candles: [{ candle: candle("2026-01-01T00:00:00Z"), final: false }],
	};
	let states = applyServerMessage({}, hourly, "BTC", "connected");
	states = applyServerMessage(states, daily, "BTC", "connected");
	expect(states["BTC:1h"]?.candles[0]?.candle.open_time).toBe(
		"2026-01-01T23:00:00Z",
	);
	expect(states["BTC:1d"]?.candles[0]?.candle.open_time).toBe(
		"2026-01-01T00:00:00Z",
	);
	states = applyServerMessage(
		states,
		{
			type: "update",
			symbol: "BTC",
			interval: "1d",
			candle: { candle: candle("2026-01-01T00:00:00Z"), final: true },
		},
		"BTC",
		"connected",
	);
	expect(states["BTC:1d"]?.candles[0]?.final).toBe(true);
	expect(states["BTC:1h"]?.candles[0]?.final).toBe(false);
});
