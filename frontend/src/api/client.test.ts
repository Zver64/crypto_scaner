import { describe, expect, it, vi } from "vitest";
import { ApiError, fetchMarketScan } from "./client";

const criteria = {
	minimumRangePercent: 3.5,
	percentile: 80,
	periodDays: 30,
};

describe("fetchMarketScan", () => {
	it("requests the protected market endpoint and preserves response values and order", async () => {
		const body = {
			analyzed_count: 3,
			insufficient_data_count: 1,
			items: [
				{ candle_count: 30, range_percent: 9.4381, symbol: "ZZZUSDT" },
				{ candle_count: 29, range_percent: 4, symbol: "AAAUSDT" },
			],
			matched_count: 2,
			minimum_range_percent: 3.5,
			percentile: 80,
			period_days: 30,
		};
		const request = vi.fn(
			async (_input: RequestInfo | URL, _init?: RequestInit) =>
				new Response(JSON.stringify(body), {
					headers: { "Content-Type": "application/json" },
					status: 200,
				}),
		);

		const result = await fetchMarketScan(criteria, {
			initData: "signed-fixture",
			request,
		});

		expect(request).toHaveBeenCalledWith(
			"/api/v1/analysis/percentile?period_days=30&percentile=80&minimum_range_percent=3.5",
			{
				headers: {
					Accept: "application/json",
					Authorization: "tma signed-fixture",
				},
				method: "GET",
			},
		);
		expect(result).toEqual(body);
		expect(result.items.map((item) => item.symbol)).toEqual([
			"ZZZUSDT",
			"AAAUSDT",
		]);
	});

	it("does not send an authorization header when local Vite must provide it", async () => {
		const request = vi.fn(
			async (_input: RequestInfo | URL, _init?: RequestInit) =>
				new Response(
					JSON.stringify({
						analyzed_count: 0,
						insufficient_data_count: 0,
						items: [],
						matched_count: 0,
						minimum_range_percent: 3.5,
						percentile: 80,
						period_days: 30,
					}),
					{ status: 200 },
				),
		);

		const result = await fetchMarketScan(criteria, { request });

		expect(result.items).toEqual([]);
		expect(request.mock.calls[0]?.[1]?.headers).toEqual({
			Accept: "application/json",
		});
	});

	it("normalizes canonical backend failures", async () => {
		const request = vi.fn(
			async () =>
				new Response(
					JSON.stringify({
						error: {
							code: "market_data_unavailable",
							message: "Market data is unavailable",
						},
						request_id: "request-123",
					}),
					{ status: 503 },
				),
		);

		await expect(fetchMarketScan(criteria, { request })).rejects.toEqual(
			expect.objectContaining({
				code: "market_data_unavailable",
				message: "Market data is not ready yet. Try again shortly.",
				requestId: "request-123",
			}),
		);
	});

	it("normalizes network and unexpected response failures", async () => {
		const offline = vi.fn(async () => {
			throw new TypeError("Failed to fetch");
		});
		await expect(
			fetchMarketScan(criteria, { request: offline }),
		).rejects.toEqual(
			expect.objectContaining({
				code: "network_error",
				message:
					"Unable to reach the scanner. Check your connection and try again.",
			}),
		);

		const malformed = vi.fn(
			async () =>
				new Response(JSON.stringify({ items: "not-an-array" }), {
					status: 200,
				}),
		);
		await expect(
			fetchMarketScan(criteria, { request: malformed }),
		).rejects.toBeInstanceOf(ApiError);
		await expect(
			fetchMarketScan(criteria, { request: malformed }),
		).rejects.toEqual(
			expect.objectContaining({
				code: "unexpected_error",
				message: "An unexpected error occurred. Please try again.",
			}),
		);
	});
});
