import { describe, expect, it, vi } from "vitest";
import { candleHistoryQueryKey, fetchCandlePage } from "@/api/candle-history";
import { ApiError } from "@/api/client";

const candle = {
	open_time: "2026-08-01T00:00:00Z",
	close_time: "2026-08-01T00:59:59.999Z",
	open: 1,
	high: 3,
	low: 0.5,
	close: 2,
	volume: 10,
	quote_asset_volume: 20,
	trade_count: 4,
};

describe("fetchCandlePage", () => {
	it("requests an interval cursor page and parses chronological candles", async () => {
		const request = vi.fn(
			async (_input: RequestInfo | URL, _init?: RequestInit) =>
				new Response(
					JSON.stringify({
						symbol: "BTCUSDT",
						interval: "1h",
						candles: [candle],
						has_more: true,
						next_before: candle.open_time,
					}),
				),
		);
		const result = await fetchCandlePage("BTCUSDT", "1h", {
			before: "2026-08-02T00:00:00Z",
			initData: "signed",
			limit: 200,
			request,
		});
		expect(result.candles[0]?.open_time).toBe("2026-08-01T00:00:00.000Z");
		expect(result.next_before).toBe("2026-08-01T00:00:00.000Z");
		const [url, init] = request.mock.calls[0] ?? [];
		expect(url).toContain(
			"interval=1h&before=2026-08-02T00%3A00%3A00Z&limit=200",
		);
		expect(init?.headers).toEqual({
			Accept: "application/json",
			Authorization: "tma signed",
		});
	});

	it.each([
		[401, "unauthenticated"],
		[403, "access_denied"],
	] as const)("preserves a %i backend error as %s", async (status, code) => {
		await expect(
			fetchCandlePage("BTCUSDT", "1h", {
				request: async () =>
					Response.json(
						{ error: { code }, request_id: "request-123" },
						{ status },
					),
			}),
		).rejects.toMatchObject({ code, requestId: "request-123", status });
	});

	it.each([
		{ interval: "1d", candles: [candle], has_more: true },
		{ interval: "1d", candles: [{ ...candle, high: 1 }], has_more: false },
		{
			interval: "1d",
			candles: [candle],
			has_more: false,
			next_before: candle.open_time,
		},
	])("rejects malformed page %#", async (payload) => {
		await expect(
			fetchCandlePage("BTCUSDT", "1d", {
				request: async () =>
					new Response(JSON.stringify({ symbol: "BTCUSDT", ...payload })),
			}),
		).rejects.toBeInstanceOf(ApiError);
	});
});

it("isolates cached pages by symbol and interval", () => {
	expect(candleHistoryQueryKey("BTCUSDT", "1h")).not.toEqual(
		candleHistoryQueryKey("BTCUSDT", "1d"),
	);
});
