import { afterEach, expect, it, vi } from "vitest";
import type {
	Candle,
	ChartPageResponse,
	LiveCandleClientMessage,
} from "@/api/generated/models";
import { createCoinChartData } from "@/features/instrument-analysis/coin-chart-data";

class FakeSocket {
	static OPEN = 1;
	static current: FakeSocket | undefined;
	readyState = FakeSocket.OPEN;
	sent: LiveCandleClientMessage[] = [];
	private onMessage?: (event: { data: string }) => void;
	constructor() {
		FakeSocket.current = this;
	}
	addEventListener(type: string, callback: (event: { data: string }) => void) {
		if (type === "message") this.onMessage = callback;
	}
	send(data: string) {
		this.sent.push(JSON.parse(data));
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

it("renders backend snapshots, merges current-candle tails, and extends the range over WebSocket", () => {
	vi.stubGlobal("window", { location: { href: "https://example.com/coin" } });
	vi.stubGlobal("WebSocket", FakeSocket);
	const fetch = vi.fn();
	vi.stubGlobal("fetch", fetch);
	const source = createCoinChartData("btcusdt");
	try {
		source.start();
		const socket = FakeSocket.current;
		if (!socket) throw new Error("Missing socket");
		socket.emitMessage({ type: "authenticated" });
		expect(
			socket.sent.map(({ type, interval, limit }) => [type, interval, limit]),
		).toEqual([
			["subscribe", "1h", 200],
			["subscribe", "1d", 200],
			["subscribe", "1w", 200],
			["subscribe", "1M", 200],
		]);
		expect(source.getSnapshot("1h").isLoading).toBe(true);

		const base = { type: "snapshot", symbol: "BTCUSDT", interval: "1h" };
		socket.emitMessage({
			...base,
			version: 1,
			chart: chart([candle(1), candle(2, 14)], [60, 70], true),
		});
		const daily = source.getSnapshot("1d");
		expect(source.getSnapshot("1h").indicator.map((p) => p.value)).toEqual([
			60, 70,
		]);

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
		expect(source.getSnapshot("1h").indicator.map((p) => p.value)).toEqual([
			60, 72,
		]);
		// An update without its preceding version is never applied.
		socket.emitMessage({
			...base,
			type: "update",
			version: 4,
			chart: chart([candle(2, 99)], [99], true),
		});
		expect(source.getSnapshot("1h").indicator.at(-1)?.value).toBe(72);

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
		expect(source.getSnapshot("1h").indicator.map((p) => p.value)).toEqual([
			50, 61, 73,
		]);
		expect(source.getSnapshot("1d")).toBe(daily);
		expect(fetch).not.toHaveBeenCalled();
	} finally {
		source.stop();
	}
});
