import { describe, expect, it, vi } from "vitest";
import { ApiError, fetchInstrumentAnalysis, fetchMarketScan } from "./client";

const criteria = {
	name: "percentile",
	parameters: {
		minimum_range_percent: 3.5,
		percentile: 80,
		period: 30,
		unit: "days",
	},
};

const evaluation = {
	candle_count: 30,
	from: "2026-07-05T00:00:00Z",
	matched: true,
	metrics: { range_percent: 4 },
	name: "percentile",
	to: "2026-08-03T00:00:00Z",
};

describe("fetchMarketScan", () => {
	it("requests the protected market endpoint and preserves response values and order", async () => {
		const body = {
			analyzed_count: 3,
			insufficient_data_count: 1,
			items: [
				{ evaluations: [evaluation], matched: true, symbol: "ZZZUSDT" },
				{ evaluations: [evaluation], matched: true, symbol: "AAAUSDT" },
			],
			matched_count: 2,
		};
		const request = vi.fn(
			async (_input: RequestInfo | URL, _init?: RequestInit) =>
				new Response(JSON.stringify(body), {
					headers: { "Content-Type": "application/json" },
					status: 200,
				}),
		);

		const result = await fetchMarketScan([criteria], {
			initData: "signed-fixture",
			request,
		});

		expect(request).toHaveBeenCalledWith("/api/v1/analysis/market", {
			body: JSON.stringify({ criteria: [criteria] }),
			headers: {
				Accept: "application/json",
				Authorization: "tma signed-fixture",
				"Content-Type": "application/json",
			},
			method: "POST",
		});
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
					}),
					{ status: 200 },
				),
		);

		const result = await fetchMarketScan([criteria], { request });

		expect(result.items).toEqual([]);
		expect(request.mock.calls[0]?.[1]?.headers).toEqual({
			Accept: "application/json",
			"Content-Type": "application/json",
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

		await expect(fetchMarketScan([criteria], { request })).rejects.toEqual(
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
			fetchMarketScan([criteria], { request: offline }),
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
			fetchMarketScan([criteria], { request: malformed }),
		).rejects.toBeInstanceOf(ApiError);
		await expect(
			fetchMarketScan([criteria], { request: malformed }),
		).rejects.toEqual(
			expect.objectContaining({
				code: "unexpected_error",
				message: "An unexpected error occurred. Please try again.",
			}),
		);
	});

	it("rejects evaluations with invalid metric or match fields", async () => {
		const request = vi.fn(
			async () =>
				new Response(
					JSON.stringify({
						analyzed_count: 1,
						insufficient_data_count: 0,
						items: [
							{
								evaluations: [
									{
										...evaluation,
										matched: "true",
										metrics: { range_percent: "4" },
									},
								],
								matched: true,
								symbol: "BTCUSDT",
							},
						],
						matched_count: 1,
					}),
					{ status: 200 },
				),
		);

		await expect(fetchMarketScan([criteria], { request })).rejects.toEqual(
			expect.objectContaining({ code: "unexpected_error" }),
		);
	});

	it.each([
		["invalid RFC3339 timestamp", { from: "2026-02-30T00:00:00Z" }],
		["non-UTC offset", { to: "2026-08-03T00:00:00+01:00" }],
	])("rejects evaluations with a %s", async (_description, timestamp) => {
		const request = vi.fn(
			async () =>
				new Response(
					JSON.stringify({
						analyzed_count: 1,
						insufficient_data_count: 0,
						items: [
							{
								evaluations: [{ ...evaluation, ...timestamp }],
								matched: true,
								symbol: "BTCUSDT",
							},
						],
						matched_count: 1,
					}),
					{ status: 200 },
				),
		);

		await expect(fetchMarketScan([criteria], { request })).rejects.toEqual(
			expect.objectContaining({ code: "unexpected_error" }),
		);
	});
});

describe("fetchInstrumentAnalysis", () => {
	it("URL-encodes the exact symbol and maps UTC coverage without changing values", async () => {
		const body = {
			evaluations: [evaluation],
			matched: true,
			symbol: "币安/USDT",
		};
		const request = vi.fn(
			async (_input: RequestInfo | URL, _init?: RequestInit) =>
				new Response(JSON.stringify(body), { status: 200 }),
		);

		const result = await fetchInstrumentAnalysis("币安/USDT", [criteria], {
			initData: "signed-fixture",
			request,
		});

		expect(request).toHaveBeenCalledWith(
			"/api/v1/analysis/instruments/%E5%B8%81%E5%AE%89%2FUSDT",
			{
				body: JSON.stringify({ criteria: [criteria] }),
				headers: {
					Accept: "application/json",
					Authorization: "tma signed-fixture",
					"Content-Type": "application/json",
				},
				method: "POST",
			},
		);
		expect(result).toEqual(body);
	});

	it("normalizes canonical instrument failures", async () => {
		const request = vi.fn(
			async () =>
				new Response(
					JSON.stringify({
						error: { code: "insufficient_data" },
						request_id: "request-456",
					}),
					{ status: 409 },
				),
		);

		await expect(
			fetchInstrumentAnalysis("BTCUSDT", [criteria], { request }),
		).rejects.toEqual(
			expect.objectContaining({
				code: "insufficient_data",
				message: "There is not enough market history for this analysis.",
				requestId: "request-456",
			}),
		);
	});
});
