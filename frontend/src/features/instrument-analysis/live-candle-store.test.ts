import { expect, it, vi } from "vitest";
import type {
	Candle,
	ChartIndicatorDefinition,
	LiveCandleServerMessage,
} from "@/api/generated/models";
import {
	applyServerMessage,
	createLiveStore,
} from "@/features/instrument-analysis/live-candle-store";

const rsiCatalog: ChartIndicatorDefinition[] = [
	{
		id: "rsi-14",
		lines: [{ color: "blue.5", output: "rsi", title: "RSI 14" }],
		parameters: { period: 14 },
		placement: "pane",
		scale: { levels: [], max: 100, min: 0 },
		type: "rsi",
	},
];

const rsiCatalogs = {
	"1h": rsiCatalog,
	"1d": rsiCatalog,
	"1w": rsiCatalog,
	"1M": rsiCatalog,
};

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
	const store = createLiveStore("BTC", rsiCatalogs);
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
		rsiCatalog,
	);
	states = applyServerMessage(
		states,
		snapshot("1d", "2026-01-01T00:00:00Z", 1, 55),
		"BTC",
		"connected",
		rsiCatalog,
	);
	const stable = states;
	states = applyServerMessage(
		states,
		snapshot("1h", "2026-01-01T23:00:00Z", 1, 99),
		"BTC",
		"connected",
		rsiCatalog,
	);
	expect(states).toBe(stable);
	states = applyServerMessage(
		states,
		snapshot("1d", "2026-01-01T00:00:00Z", 2, 60),
		"BTC",
		"connected",
		rsiCatalog,
	);
	expect(
		states["BTC:1h"]?.chart?.indicators[0]?.series[0]?.points[0]?.value,
	).toBe(45);
	expect(
		states["BTC:1d"]?.chart?.indicators[0]?.series[0]?.points[0]?.value,
	).toBe(60);
});

it("asks once for a new snapshot when an update cannot be applied", () => {
	const onResync = vi.fn();
	const store = createLiveStore("BTC", rsiCatalogs, onResync);
	const time = "2026-01-01T23:00:00Z";
	const update = (version: number): LiveCandleServerMessage => ({
		...snapshot("1h", time, version, 50),
		type: "update",
	});
	store.message(snapshot("1h", time, 1, 45));
	store.message(update(3));
	store.message(update(4));
	expect(onResync).toHaveBeenCalledOnce();
	expect(onResync).toHaveBeenCalledWith("1h");
	expect(store.getSnapshot("1h").freshness).toBe("stale");

	store.message(snapshot("1h", time, 5, 55));
	store.message(update(6));
	expect(store.getSnapshot("1h")).toMatchObject({
		freshness: "fresh",
		resyncing: false,
		version: 6,
	});
	expect(onResync).toHaveBeenCalledOnce();
});
