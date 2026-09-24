import { QueryClient } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { getGetInstrumentChartInfiniteQueryKey } from "@/api/generated/api";
import { rsiChartRequest } from "@/features/instrument-analysis/chart-page";
import { createCoinChartData } from "@/features/instrument-analysis/coin-chart-data";

class FakeSocket {
	static OPEN = 1;
	static current: FakeSocket | undefined;
	readyState = FakeSocket.OPEN;
	private onMessage?: (event: { data: string }) => void;
	constructor() {
		FakeSocket.current = this;
	}
	addEventListener(type: string, callback: (event: { data: string }) => void) {
		if (type === "message") this.onMessage = callback;
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

function stubChartTransport() {
	vi.stubGlobal("window", { location: { href: "https://example.com/coin" } });
	vi.stubGlobal("WebSocket", FakeSocket);
	const requested: string[] = [];
	vi.stubGlobal(
		"fetch",
		vi.fn(async (url: string) => {
			const interval =
				new URL(url, "https://example.com").searchParams.get("interval") ??
				"1h";
			requested.push(interval);
			return new Response(
				JSON.stringify({
					symbol: "BTCUSDT",
					interval,
					candles: [
						{
							open_time: "2026-08-27T00:00:00Z",
							close_time: "2026-08-27T00:59:59.999Z",
							open: 10,
							high: 13,
							low: 9,
							close: 12,
							volume: 10,
							quote_asset_volume: 20,
							trade_count: 4,
						},
					],
					has_more: false,
					indicators: [
						{
							type: "rsi",
							parameters: { period: 14 },
							series: [
								{
									name: "rsi",
									points: [{ time: "2026-08-27T00:00:00Z", value: 62.5 }],
								},
							],
						},
					],
				}),
				{ status: 200 },
			);
		}),
	);
	return requested;
}

it("loads the visible interval first, then prepares the other intervals without updating its snapshot", async () => {
	const requested = stubChartTransport();
	const queryClient = new QueryClient();
	const source = createCoinChartData("BTCUSDT", queryClient);
	try {
		source.start("1h");
		expect(requested).toEqual(["1h"]);
		await vi.waitFor(() =>
			expect(source.getSnapshot("1h").candles).toHaveLength(1),
		);
		const hourly = source.getSnapshot("1h");
		await vi.waitFor(() => expect(requested).toHaveLength(4));
		expect(requested.sort()).toEqual(["1M", "1d", "1h", "1w"]);
		expect(source.getSnapshot("1h")).toBe(hourly);
		await vi.waitFor(() =>
			expect(source.getSnapshot("1d").candles).toHaveLength(1),
		);
	} finally {
		source.stop();
		queryClient.clear();
	}
});

it("keeps the visible snapshot stable when query status changes without new data", async () => {
	stubChartTransport();
	const queryClient = new QueryClient();
	const source = createCoinChartData("BTCUSDT", queryClient);
	try {
		source.start("1h");
		await vi.waitFor(() =>
			expect(source.getSnapshot("1M").candles).toHaveLength(1),
		);
		const hourly = source.getSnapshot("1h");
		const query = queryClient.getQueryCache().find({
			queryKey: getGetInstrumentChartInfiniteQueryKey(
				"BTCUSDT",
				rsiChartRequest,
				{ interval: "1h", limit: 200 },
			),
		});
		expect(query).toBeDefined();
		query?.setState({ fetchStatus: "fetching" });
		query?.setState({ fetchStatus: "idle" });
		expect(source.getSnapshot("1h")).toBe(hourly);
	} finally {
		source.stop();
		queryClient.clear();
	}
});

it("aborts pending history on stop and starts a fresh observer on restart", () => {
	stubChartTransport();
	const signals: AbortSignal[] = [];
	vi.stubGlobal(
		"fetch",
		vi.fn((_url: string, options: RequestInit) => {
			const signal = options.signal;
			if (!signal) throw new Error("Missing request signal");
			signals.push(signal);
			return new Promise<Response>((_resolve, reject) => {
				signal.addEventListener("abort", () =>
					reject(new DOMException("Aborted", "AbortError")),
				);
			});
		}),
	);
	const queryClient = new QueryClient();
	const source = createCoinChartData("BTCUSDT", queryClient);
	try {
		source.start("1h");
		expect(signals).toHaveLength(1);
		source.stop();
		expect(signals[0]?.aborted).toBe(true);
		source.start("1h");
		expect(signals).toHaveLength(2);
		expect(signals[1]?.aborted).toBe(false);
	} finally {
		source.stop();
		queryClient.clear();
	}
	expect(signals[1]?.aborted).toBe(true);
});

it("aborts an in-flight head refresh on stop and ignores a late response", async () => {
	stubChartTransport();
	const queryClient = new QueryClient();
	const source = createCoinChartData("BTCUSDT", queryClient);
	try {
		source.start("1h");
		await vi.waitFor(() =>
			expect(source.getSnapshot("1M").candles).toHaveLength(1),
		);
		let headSignal: AbortSignal | undefined;
		let resolveHead: ((value: Response) => void) | undefined;
		vi.stubGlobal(
			"fetch",
			vi.fn((_url: string, options: RequestInit) => {
				headSignal = options.signal ?? undefined;
				return new Promise<Response>((resolve) => {
					resolveHead = resolve;
				});
			}),
		);
		FakeSocket.current?.emitMessage({
			type: "update",
			symbol: "BTCUSDT",
			interval: "1h",
			candle: {
				candle: {
					open_time: "2026-08-27T00:00:00Z",
					close_time: "2026-08-27T00:59:59.999Z",
					open: 10,
					high: 13,
					low: 9,
					close: 12,
					volume: 10,
					quote_asset_volume: 20,
					trade_count: 4,
				},
				final: true,
			},
		});
		await vi.waitFor(() => expect(headSignal).toBeDefined());
		source.stop();
		const stopped = source.getSnapshot("1h");
		expect(headSignal?.aborted).toBe(true);
		resolveHead?.(new Response("{}", { status: 200 }));
		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(source.getSnapshot("1h")).toBe(stopped);
	} finally {
		source.stop();
		queryClient.clear();
	}
});

it("restores cached chart pages and prepares all intervals on reopening without a refetch", async () => {
	const requested = stubChartTransport();
	const queryClient = new QueryClient();
	const first = createCoinChartData("BTCUSDT", queryClient);
	let reopened: ReturnType<typeof createCoinChartData> | undefined;
	try {
		first.start("1h");
		await vi.waitFor(() =>
			expect(first.getSnapshot("1M").candles).toHaveLength(1),
		);
		first.stop();
		expect(requested).toHaveLength(4);

		reopened = createCoinChartData("BTCUSDT", queryClient);
		reopened.start("1h");
		await vi.waitFor(() =>
			expect(reopened?.getSnapshot("1h").candles).toHaveLength(1),
		);
		await vi.waitFor(() =>
			expect(reopened?.getSnapshot("1d").candles).toHaveLength(1),
		);
		expect(requested).toHaveLength(4);
	} finally {
		reopened?.stop();
		first.stop();
		queryClient.clear();
	}
});
