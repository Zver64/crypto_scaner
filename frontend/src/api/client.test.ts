import { describe, expect, it, vi } from "vitest";
import { ApiError, fetchInstrumentAnalysis, fetchMarketScan } from "./client";

const criteria = {
	key: "volatility",
	label: "Volatility",
	name: "volatility",
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
	key: "volatility",
	label: "Volatility",
	matched: true,
	metrics: { range_percent: 4 },
	name: "volatility",
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
			unresolved: [],
			warnings: [],
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
						unresolved: [],
						warnings: [],
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
						unresolved: [],
						warnings: [],
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
						unresolved: [],
						warnings: [],
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
	it("submits Daily Volatility, Hourly Volatility, then enabled criteria", async () => {
		const { criterionSelections, defaultMarketScanCriteria } = await import(
			"@/features/market-scan/pipeline"
		);
		const selections = criterionSelections(defaultMarketScanCriteria);
		const body = {
			evaluations: [
				{
					...evaluation,
					key: "daily_volatility",
					label: "Daily Volatility",
				},
				{
					...evaluation,
					key: "hourly_volatility",
					label: "Hourly Volatility",
				},
			],
			matched: true,
			symbol: "BTCUSDT",
			warnings: [],
		};
		const request = vi.fn(
			async (_input: RequestInfo | URL, init?: RequestInit) => {
				expect(JSON.parse(String(init?.body))).toMatchObject({
					criteria: [
						{ key: "daily_volatility", name: "volatility" },
						{ key: "hourly_volatility", name: "volatility" },
						{ key: "market_cap", name: "market_cap" },
					],
				});
				return new Response(JSON.stringify(body), { status: 200 });
			},
		);

		expect(
			await fetchInstrumentAnalysis("BTCUSDT", selections, { request }),
		).toEqual(body);
	});

	it("URL-encodes the exact symbol and maps UTC coverage without changing values", async () => {
		const body = {
			evaluations: [evaluation],
			matched: true,
			symbol: "币安/USDT",
			warnings: [
				{
					code: "market_cap_provider_unavailable",
					message: "Cached values were used",
				},
			],
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

	it("maps dynamic Market Cap resolution errors to an available message", async () => {
		const request = vi.fn(
			async () =>
				new Response(JSON.stringify({ error: { code: "mapping_not_found" } }), {
					status: 422,
				}),
		);

		await expect(
			fetchInstrumentAnalysis("BTCUSDT", [criteria], { request }),
		).rejects.toEqual(
			expect.objectContaining({
				code: "market_cap_unavailable",
				message:
					"Market capitalization is unavailable for this instrument or is still loading. Try again shortly.",
			}),
		);
	});
});

it("parses Market Cap warnings and unresolved instruments", async () => {
	const request = vi.fn(
		async () =>
			new Response(
				JSON.stringify({
					analyzed_count: 1,
					insufficient_data_count: 0,
					items: [],
					matched_count: 0,
					unresolved: [
						{
							code: "mapping_not_found",
							message: "Market capitalization could not be resolved",
							symbol: "UNKNOWNUSDT",
						},
					],
					warnings: [
						{
							code: "market_cap_provider_unavailable",
							message: "Cached values were used",
						},
					],
				}),
				{ status: 200 },
			),
	);

	await expect(fetchMarketScan([criteria], { request })).resolves.toMatchObject(
		{
			unresolved: [{ code: "mapping_not_found", symbol: "UNKNOWNUSDT" }],
			warnings: [{ code: "market_cap_provider_unavailable" }],
		},
	);
});

it("rejects unknown unresolved instrument codes", async () => {
	const request = vi.fn(
		async () =>
			new Response(
				JSON.stringify({
					analyzed_count: 0,
					insufficient_data_count: 0,
					items: [],
					matched_count: 0,
					unresolved: [
						{
							code: "unknown",
							message: "Unavailable",
							symbol: "UNKNOWNUSDT",
						},
					],
					warnings: [],
				}),
				{ status: 200 },
			),
	);

	await expect(fetchMarketScan([criteria], { request })).rejects.toEqual(
		expect.objectContaining({ code: "unexpected_error" }),
	);
});

it("submits the unified pipeline and preserves both keyed outcomes, warnings, and unresolved instruments", async () => {
	const { criterionSelections, defaultMarketScanCriteria } = await import(
		"@/features/market-scan/pipeline"
	);
	const selections = criterionSelections(defaultMarketScanCriteria);
	const body = {
		matched_count: 1,
		analyzed_count: 2,
		insufficient_data_count: 0,
		items: [
			{
				symbol: "BTCUSDT",
				matched: true,
				evaluations: [
					{
						...evaluation,
						key: "daily_volatility",
						label: "Daily Volatility",
						metrics: { range_percent: 6 },
					},
					{
						...evaluation,
						key: "hourly_volatility",
						label: "Hourly Volatility",
						candle_count: 60,
						metrics: { range_percent: 2 },
					},
					{
						...evaluation,
						key: "market_cap",
						name: "market_cap",
						label: "Market Cap",
						candle_count: 0,
						metrics: { market_cap_usd: 500_000_000 },
					},
				],
			},
		],
		unresolved: [
			{
				symbol: "UNKNOWNUSDT",
				code: "mapping_not_found",
				message: "Mapping not found",
			},
		],
		warnings: [
			{
				code: "market_cap_provider_unavailable",
				message: "Cached values used",
			},
		],
	};
	const request = vi.fn(
		async (_input: RequestInfo | URL, init?: RequestInit) => {
			expect(JSON.parse(String(init?.body))).toEqual({
				criteria: [
					{
						key: "daily_volatility",
						name: "volatility",
						label: "Daily Volatility",
						parameters: {
							unit: "days",
							period: 30,
							percentile: 80,
							minimum_range_percent: 5,
						},
					},
					{
						key: "hourly_volatility",
						name: "volatility",
						label: "Hourly Volatility",
						parameters: {
							unit: "hours",
							period: 60,
							percentile: 80,
							minimum_range_percent: 2,
						},
					},
					{
						key: "market_cap",
						name: "market_cap",
						label: "Market Cap",
						parameters: { min_market_cap_usd: 500_000_000 },
					},
				],
			});
			return new Response(JSON.stringify(body), { status: 200 });
		},
	);
	expect(await fetchMarketScan(selections, { request })).toEqual(body);
});
