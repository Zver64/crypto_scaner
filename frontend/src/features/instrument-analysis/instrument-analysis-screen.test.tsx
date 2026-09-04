import { MantineProvider } from "@mantine/core";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, expect, it, vi } from "vitest";
import type {
	CriterionSelection,
	InstrumentAnalysisResult,
} from "@/api/client";
import {
	instrumentAnalysisQueryKey,
	instrumentAnalysisQueryOptions,
} from "@/api/instrument-analysis";
import { BusinessRequestContext } from "@/app/business-request-context";
import { InstrumentAnalysisScreen } from "@/features/instrument-analysis/instrument-analysis-screen";
import {
	criterionSelections,
	defaultMarketScanCriteria,
} from "@/features/market-scan/pipeline";

function renderAnalysis(
	result: InstrumentAnalysisResult | undefined,
	selections: readonly CriterionSelection[] = criterionSelections(
		defaultMarketScanCriteria,
	),
	client = new QueryClient(),
) {
	vi.stubGlobal("window", {});
	// Render the settled HTTP state without starting a new request on mount.
	client.setQueryDefaults(instrumentAnalysisQueryKey("BTCUSDT", selections), {
		retryOnMount: false,
	});
	if (result)
		client.setQueryData(
			instrumentAnalysisQueryKey("BTCUSDT", selections),
			result,
		);
	return renderToStaticMarkup(
		<MantineProvider>
			<QueryClientProvider client={client}>
				<BusinessRequestContext
					value={{ allowed: true, authenticated: true, backendReady: true }}
				>
					<InstrumentAnalysisScreen
						criterionSelections={selections}
						onBack={() => {}}
						symbol="BTCUSDT"
					/>
				</BusinessRequestContext>
			</QueryClientProvider>
		</MantineProvider>,
	);
}

const activeQueryClients = new Set<QueryClient>();

function createHTTPAnalysisHarness() {
	vi.stubGlobal("window", {});
	const selections = criterionSelections(defaultMarketScanCriteria);
	const client = new QueryClient();
	activeQueryClients.add(client);
	return {
		client,
		query: (staleTime?: number) =>
			client.fetchQuery({
				...instrumentAnalysisQueryOptions("BTCUSDT", selections),
				...(staleTime === undefined ? {} : { staleTime }),
			}),
		renderText: () =>
			renderAnalysis(undefined, selections, client).replace(/<[^>]*>/g, ""),
		selections,
	};
}

function stubJSONResponse(payload: unknown, status = 200) {
	vi.stubGlobal(
		"fetch",
		vi
			.fn()
			.mockResolvedValue(new Response(JSON.stringify(payload), { status })),
	);
}

afterEach(() => {
	for (const client of activeQueryClients) client.clear();
	activeQueryClients.clear();
	vi.unstubAllGlobals();
});

it("shows keyed ranges and complete sample coverage with non-default scan settings", () => {
	const daily = {
		candle_count: 14,
		from: "2026-08-01T00:00:00Z",
		key: "daily_volatility",
		label: "Daily Volatility",
		matched: true,
		metrics: { range_percent: 6.25 },
		name: "volatility",
		to: "2026-08-30T00:00:00Z",
	};
	const hourly = {
		...daily,
		candle_count: 48,
		key: "hourly_volatility",
		label: "Hourly Volatility",
		metrics: { range_percent: 2.75 },
	};
	const html = renderAnalysis(
		{
			evaluations: [
				hourly,
				daily,
				{
					candle_count: 0,
					from: "0001-01-01T00:00:00Z",
					key: "market_cap",
					label: "Market Cap",
					matched: true,
					metrics: { market_cap_usd: 1_500_000_000 },
					name: "market_cap",
					to: "0001-01-01T00:00:00Z",
				},
			],
			matched: true,
			symbol: "BTCUSDT",
			warnings: [],
		},
		criterionSelections({
			...defaultMarketScanCriteria,
			period: 14,
			percentile: 65,
			hourlyPeriod: 48,
			hourlyPercentile: 95,
		}).reverse(),
	);

	expect(html).toContain("Symbol");
	expect(html).toContain("BTCUSDT");
	expect(html).toContain("Market Cap");
	expect(html).toContain("$1.5B");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Daily Grid Step3.13%");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Hourly Grid Step1.38%");
	expect(html).not.toContain("Incomplete sample");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Daily Range6.25%");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Hourly Range2.75%");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Days available: 14 of 14");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Hours available: 48 of 48");
	expect(html).not.toContain("Daily Volatility");
	expect(html).not.toContain("Hourly Volatility");
	expect(html).not.toContain("Coverage From");
	expect(html).not.toContain("Coverage To");
	expect(html).not.toContain('type="radio"');
	expect(html).not.toContain("Minimum Market Cap");
	expect(html).not.toContain("Recalculate Instrument");
	expect(html).not.toContain("Matched");
	expect(html).not.toContain("Instrument Analysis");
});

it("keeps the summary visible when volatility short-circuits market cap", () => {
	const html = renderAnalysis({
		evaluations: [
			{
				candle_count: 30,
				from: "2026-08-01T00:00:00Z",
				key: "daily_volatility",
				label: "Daily Volatility",
				matched: false,
				metrics: { range_percent: 4 },
				name: "volatility",
				to: "2026-08-30T00:00:00Z",
			},
		],
		matched: false,
		symbol: "BTCUSDT",
		warnings: [],
	});

	expect(html).toContain("Symbol");
	expect(html).not.toContain("Matched");
	expect(html).not.toContain("Daily Volatility");
	expect(html).not.toContain("Hourly Volatility");
	const text = html.replace(/<[^>]*>/g, "");
	expect(text).toContain("Daily Range4%");
	expect(text).toContain("Daily Grid Step2%");
	expect(text).toContain("Hourly Grid StepNot enough data");
	expect(text).toContain("Days available: 30 of 30");
	expect(text).toContain("Hourly Range—");
	expect(text).toContain("Hours available: —");
	expect(text).not.toContain("Market Cap");
});

it("shows partial samples and caps available counts at the requested period", () => {
	const daily = {
		candle_count: 9,
		from: "2026-08-01T00:00:00Z",
		key: "daily_volatility",
		label: "Daily Volatility",
		matched: true,
		metrics: { range_percent: 0.01234 },
		name: "volatility",
		to: "2026-08-09T00:00:00Z",
	};
	const html = renderAnalysis(
		{
			evaluations: [
				daily,
				{ ...daily, key: "hourly_volatility", candle_count: 100 },
			],
			matched: true,
			symbol: "BTCUSDT",
			warnings: [],
		},
		criterionSelections({
			...defaultMarketScanCriteria,
			period: 14,
			hourlyPeriod: 48,
		}),
	);

	expect(html.replace(/<[^>]*>/g, "")).toContain("Days available: 9 of 14");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Hours available: 48 of 48");
	expect(html).toContain("0.0123%");
	const text = html.replace(/<[^>]*>/g, "");
	expect(text).toContain("Daily Grid Step0.00617%Incomplete sample: 9 of 14");
	expect(text).toContain("Hourly Grid Step0.00617%");
	expect(text.match(/Incomplete sample/g)).toHaveLength(1);
});

it.each([
	undefined,
	null,
	-1,
	Number.NaN,
])("keeps missing metrics and unavailable counts (%s) distinct from zero", (count) => {
	const daily = {
		candle_count: 0,
		from: "2026-08-01T00:00:00Z",
		key: "daily_volatility",
		label: "Daily Volatility",
		matched: true,
		metrics: { range_percent: 0 },
		name: "volatility",
		to: "2026-08-01T00:00:00Z",
	};
	const html = renderAnalysis({
		evaluations: [
			daily,
			{
				...daily,
				key: "hourly_volatility",
				// Exercise defensive rendering of an unavailable sample count.
				candle_count: count as number,
				metrics: {},
			},
		],
		matched: true,
		symbol: "BTCUSDT",
		warnings: [],
	});
	const text = html.replace(/<[^>]*>/g, "");
	expect(text).toContain("Daily Range0%");
	expect(text).toContain("Daily Grid Step0%");
	expect(text).toContain("Hourly Grid StepNot enough data");
	expect(text).toContain("Days available: 0 of 30");
	expect(text).toContain("Hourly Range—");
	expect(text).toContain("Hours available: —");
});

it.each([
	[24.69, "12.3%"],
	[2.469, "1.23%"],
	[0.2469, "0.123%"],
	[0.02469, "0.0123%"],
	[0.00001234, "0.00000617%"],
])("calculates Grid Steps from the original HTTP range %s before formatting", async (range, expected) => {
	const analysis = createHTTPAnalysisHarness();
	const evaluation = {
		candle_count: 30,
		from: "2026-08-01T00:00:00Z",
		key: "daily_volatility",
		label: "Daily Volatility",
		matched: false,
		metrics: { range_percent: range },
		name: "volatility",
		to: "2026-08-30T00:00:00Z",
	};
	stubJSONResponse({
		evaluations: [evaluation],
		matched: false,
		symbol: "BTCUSDT",
		warnings: [],
	});
	await analysis.query();
	const text = analysis.renderText();
	expect(text).toContain("Recommended trading bot settings");
	expect(text).toContain(`Daily Grid Step${expected}`);
	expect(text).toContain("Hourly Grid StepNot enough data");
});

it("shows unavailable recommendations for canonical insufficient_data without inventing coverage", async () => {
	const analysis = createHTTPAnalysisHarness();
	stubJSONResponse(
		{
			error: {
				code: "insufficient_data",
				message: "Not enough closed candles for the requested period",
				details: { criterion: "volatility", available: 0, required: 30 },
			},
			request_id: "history-request",
		},
		409,
	);
	await expect(analysis.query()).rejects.toMatchObject({
		code: "insufficient_data",
		status: 409,
		requestId: "history-request",
	});
	const text = analysis.renderText();
	expect(text).toContain("BTCUSDT");
	expect(text).toContain("Recommended trading bot settings");
	expect(text).toContain("Daily Grid StepNot enough data");
	expect(text).toContain("Hourly Grid StepNot enough data");
	expect(text).toContain("Days available: —");
	expect(text).toContain("Hours available: —");
	expect(text).not.toContain("0 of");
	expect(
		analysis.client.getQueryData(
			instrumentAnalysisQueryKey("BTCUSDT", analysis.selections),
		),
	).toBeUndefined();
});

it.each([
	[401, "unauthenticated"],
	[403, "access_denied"],
	[500, "internal_error"],
	[503, "market_data_unavailable"],
	[503, "market_cap_unavailable"],
	[404, "symbol_not_found"],
	[0, "network_error"],
])("does not present unrelated failure %s %s as insufficient history", async (status, code) => {
	const analysis = createHTTPAnalysisHarness();
	vi.stubGlobal(
		"fetch",
		status
			? vi.fn().mockResolvedValue(
					new Response(
						JSON.stringify({
							error: { code },
							request_id: "unrelated-error",
						}),
						{ status },
					),
				)
			: vi.fn().mockRejectedValue(new TypeError("Failed to fetch")),
	);
	await expect(analysis.query()).rejects.toBeInstanceOf(Error);
	const text = analysis.renderText();
	expect(text).not.toContain("Not enough data");
	expect(text).not.toContain("Recommended trading bot settings");
});

it("preserves valid analysis content when a refresh returns insufficient_data", async () => {
	const analysis = createHTTPAnalysisHarness();
	const result: InstrumentAnalysisResult = {
		evaluations: [
			{
				candle_count: 9,
				from: "2026-08-01T00:00:00Z",
				key: "daily_volatility",
				label: "Daily Volatility",
				matched: false,
				metrics: { range_percent: 4 },
				name: "volatility",
				to: "2026-08-09T00:00:00Z",
			},
		],
		matched: false,
		symbol: "BTCUSDT",
		warnings: [
			{
				code: "market_cap_unavailable",
				message: "Market capitalization is temporarily unavailable",
			},
		],
	};
	analysis.client.setQueryData(
		instrumentAnalysisQueryKey("BTCUSDT", analysis.selections),
		result,
	);
	stubJSONResponse({ error: { code: "insufficient_data" } }, 409);
	await expect(analysis.query(0)).rejects.toMatchObject({
		code: "insufficient_data",
	});
	const text = analysis.renderText();
	expect(text).toContain("Daily Range4%");
	expect(text).toContain("Daily Grid Step2%Incomplete sample: 9 of 30");
	expect(text).toContain("Hourly Grid StepNot enough data");
	expect(
		analysis.client.getQueryData(
			instrumentAnalysisQueryKey("BTCUSDT", analysis.selections),
		),
	).toEqual(result);
});
