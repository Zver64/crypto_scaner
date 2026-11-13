import { afterEach, expect, it, vi } from "vitest";

import type {
	Candle,
	ChartIndicatorDefinition,
	ChartPageResponse,
	LiveCandleClientMessage,
} from "@/api/generated/models";
import { createCoinChartData } from "@/features/instrument-analysis/coin-chart-data";

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

const rsiValues = (source: ReturnType<typeof createCoinChartData>) =>
	(source.getSnapshot("1h").series.lines["rsi-14:rsi"]?.data ?? []).flatMap(
		(slot) => ("value" in slot ? [slot.value] : []),
	);

vi.mock("@/api/session", () => ({
	getSessionToken: async () => "session-token",
	invalidateSessionToken: () => {},
}));

class FakeSocket {
	static OPEN = 1;
	static current: FakeSocket | undefined;
	readyState = FakeSocket.OPEN;
	sent: LiveCandleClientMessage[] = [];
	private onMessage?: (event: { data: string }) => void;
	private onOpen?: () => void;
	constructor() {
		FakeSocket.current = this;
	}
	addEventListener(type: string, callback: (event: { data: string }) => void) {
		if (type === "message") this.onMessage = callback;
		if (type === "open") this.onOpen = () => callback({ data: "" });
	}
	send(data: string) {
		this.sent.push(JSON.parse(data));
	}
	emitOpen() {
		this.onOpen?.();
	}
	emitMessage(message: unknown) {
		this.onMessage?.({ data: JSON.stringify(message) });
	}
	close() {
		this.readyState = 3;
	}
}

afterEach(() => {
	vi.unstubAllGlobals();
	FakeSocket.current = undefined;
});

function candle(hour: number, close = 12): Candle {
	const open = new Date(Date.UTC(2026, 7, 27, hour));
	return {
		open_time: open.toISOString().replace(".000Z", "Z"),
		close_time: new Date(open.getTime() + 3_599_999).toISOString(),
		open: 10,
		high: 100,
		low: 9,
		close,
		volume: 10,
		quote_asset_volume: 20,
		trade_count: 4,
	};
}

function chart(
	candles: Candle[],
	values: number[],
	hasMore: boolean,
): ChartPageResponse {
	return {
		symbol: "BTCUSDT",
		interval: "1h",
		candles,
		has_more: hasMore,
		next_before: hasMore ? (candles[0]?.open_time ?? null) : null,
		indicators: [
			{
				type: "rsi",
				parameters: { period: 14 },
				series: [
					{
						name: "rsi",
						points: values.map((value, index) => ({
							time: candles[candles.length - values.length + index]
								?.open_time as string,
							value,
						})),
					},
				],
			},
		],
	};
}

it("renders backend snapshots, merges current-candle tails, and extends the range over WebSocket", async () => {
	vi.stubGlobal("window", {
		location: { href: "https://example.com/coin" },
		Telegram: { WebApp: { initData: "signed-data" } },
	});
	vi.stubGlobal("WebSocket", FakeSocket);
	const fetch = vi.fn<typeof globalThis.fetch>();
	vi.stubGlobal("fetch", fetch);
	const source = createCoinChartData("btcusdt", rsiCatalogs);
	try {
		source.show("1h");
		source.start();
		await vi.waitFor(() => expect(FakeSocket.current).toBeDefined());
		const socket = FakeSocket.current;
		if (!socket) throw new Error("Missing socket");
		socket.emitOpen();
		expect(
			socket.sent.map(({ type, interval, limit }) => [type, interval, limit]),
		).toEqual([["subscribe", "1h", 200]]);
		expect(source.getSnapshot("1h").isLoading).toBe(true);

		const base = { type: "snapshot", symbol: "BTCUSDT", interval: "1h" };
		socket.emitMessage({
			...base,
			version: 1,
			chart: chart([candle(1), candle(2, 14)], [60, 70], true),
		});
		const daily = source.getSnapshot("1d");
		expect(rsiValues(source)).toEqual([60, 70]);

		// A trade replaces only the current candle and its RSI point.
		socket.emitMessage({
			...base,
			type: "update",
			version: 2,
			chart: chart([candle(2, 15)], [72], true),
		});
		expect(source.getSnapshot("1h").candles.map((c) => c.close)).toEqual([
			12, 15,
		]);
		expect(rsiValues(source)).toEqual([60, 72]);
		// An update without its preceding version is never applied.
		socket.emitMessage({
			...base,
			type: "update",
			version: 4,
			chart: chart([candle(2, 99)], [99], true),
		});
		expect(rsiValues(source).at(-1)).toBe(72);

		source.loadOlder("1h");
		expect(socket.sent.at(-1)).toMatchObject({
			type: "subscribe",
			interval: "1h",
			limit: 400,
		});
		expect(source.getSnapshot("1h").isLoadingMore).toBe(true);
		// The extended range replaces every RSI point with a full recalculation.
		socket.emitMessage({
			...base,
			version: 3,
			chart: chart([candle(0), candle(1), candle(2, 15)], [50, 61, 73], false),
		});
		expect(source.getSnapshot("1h").isLoadingMore).toBe(false);
		expect(rsiValues(source)).toEqual([50, 61, 73]);
		expect(source.getSnapshot("1d")).toBe(daily);
		expect(fetch).not.toHaveBeenCalled();
	} finally {
		source.stop();
	}
});

it("streams only the interval on screen and keeps the last chart of others as stale", async () => {
	vi.stubGlobal("window", {
		location: { href: "https://example.com/coin" },
		Telegram: { WebApp: { initData: "signed-data" } },
	});
	vi.stubGlobal("WebSocket", FakeSocket);
	const source = createCoinChartData("btcusdt", rsiCatalogs);
	try {
		source.show("1h");
		source.start();
		await vi.waitFor(() => expect(FakeSocket.current).toBeDefined());
		const socket = FakeSocket.current;
		if (!socket) throw new Error("Missing socket");
		socket.emitOpen();
		socket.emitMessage({
			type: "snapshot",
			symbol: "BTCUSDT",
			interval: "1h",
			version: 1,
			chart: chart([candle(1), candle(2)], [60, 70], true),
		});
		source.loadOlder("1d");
		source.show("1d");
		expect(
			socket.sent.map(({ type, interval, limit }) => [type, interval, limit]),
		).toEqual([
			["subscribe", "1h", 200],
			["unsubscribe", "1h", 200],
			["subscribe", "1d", 200],
		]);

		const { series } = source.getSnapshot("1h");
		socket.emitMessage({
			type: "unsubscribed",
			symbol: "BTCUSDT",
			interval: "1h",
		});
		const hourly = source.getSnapshot("1h");
		expect(hourly.candles).toHaveLength(2);
		expect(hourly.freshness).toBe("stale");
		// A freshness change keeps the chart-ready data, so nothing is redrawn.
		expect(hourly.series).toBe(series);
	} finally {
		source.stop();
	}
});

it("streams Bitcoin for comparison only while enabled, with the interval and range of the coin", async () => {
	vi.stubGlobal("window", {
		location: { href: "https://example.com/coin" },
		Telegram: { WebApp: { initData: "signed-data" } },
	});
	vi.stubGlobal("WebSocket", FakeSocket);
	expect(
		createCoinChartData("btcusdt", rsiCatalogs).comparison,
	).toBeUndefined();
	const source = createCoinChartData("ethusdt", rsiCatalogs);
	const comparison = source.comparison;
	if (!comparison) throw new Error("Missing comparison");
	const lineValues = () =>
		comparison
			.getLine("1h")
			.data.map((slot) => ("value" in slot ? slot.value : null));
	try {
		source.show("1h");
		source.start();
		await vi.waitFor(() => expect(FakeSocket.current).toBeDefined());
		const socket = FakeSocket.current;
		if (!socket) throw new Error("Missing socket");
		socket.emitOpen();
		const sent = () =>
			socket.sent.map(({ type, symbol, interval, limit }) => [
				type,
				symbol,
				interval,
				limit,
			]);
		expect(sent()).toEqual([["subscribe", "ETHUSDT", "1h", 200]]);

		comparison.setEnabled(true);
		expect(sent().at(-1)).toEqual(["subscribe", "BTCUSDT", "1h", 200]);
		expect(socket.sent.at(-1)?.indicators).toBeUndefined();

		const base = { type: "snapshot", interval: "1h", version: 1 };
		socket.emitMessage({
			...base,
			symbol: "ETHUSDT",
			chart: {
				...chart([candle(1), candle(3)], [60, 70], true),
				symbol: "ETHUSDT",
			},
		});
		expect(lineValues()).toEqual([]);
		// Bitcoin candles outside the coin's slots are dropped, and a slot
		// without a Bitcoin candle stays empty.
		socket.emitMessage({
			...base,
			symbol: "BTCUSDT",
			chart: {
				...chart([candle(0, 20), candle(1, 21), candle(3, 23)], [], true),
				indicators: [],
			},
		});
		expect(lineValues()).toEqual([21, null, 23]);
		expect(source.getSnapshot("1h").candles).toHaveLength(2);
		expect(comparison.getLine("1h")).toBe(comparison.getLine("1h"));

		source.loadOlder("1h");
		expect(sent().slice(-2)).toEqual([
			["subscribe", "ETHUSDT", "1h", 400],
			["subscribe", "BTCUSDT", "1h", 400],
		]);
		source.show("1d");
		expect(sent().slice(-4)).toEqual([
			["unsubscribe", "ETHUSDT", "1h", 400],
			["unsubscribe", "BTCUSDT", "1h", 400],
			["subscribe", "ETHUSDT", "1d", 200],
			["subscribe", "BTCUSDT", "1d", 200],
		]);

		comparison.setEnabled(false);
		expect(sent().at(-1)).toEqual(["unsubscribe", "BTCUSDT", "1d", 200]);
		expect(lineValues()).toEqual([]);
	} finally {
		source.stop();
	}
});
