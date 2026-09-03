import { MantineProvider } from "@mantine/core";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type {
	CriterionSelection,
	InstrumentAnalysisResult,
} from "@/api/client";
import { instrumentAnalysisQueryKey } from "@/api/instrument-analysis";
import { BusinessRequestContext } from "@/app/business-request-context";
import { InstrumentAnalysisScreen } from "@/features/instrument-analysis/instrument-analysis-screen";
import {
	criterionSelections,
	defaultMarketScanCriteria,
} from "@/features/market-scan/pipeline";

function renderAnalysis(
	result: InstrumentAnalysisResult,
	selections: readonly CriterionSelection[] = criterionSelections(
		defaultMarketScanCriteria,
	),
) {
	vi.stubGlobal("window", {});
	const client = new QueryClient();
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
	expect(html.replace(/<[^>]*>/g, "")).toContain("Daily Range6.25%");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Hourly Range2.75%");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Дней доступно: 14 из 14");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Часов доступно: 48 из 48");
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
	expect(text).toContain("Дней доступно: 30 из 30");
	expect(text).toContain("Hourly Range—");
	expect(text).toContain("Часов доступно: —");
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

	expect(html.replace(/<[^>]*>/g, "")).toContain("Дней доступно: 9 из 14");
	expect(html.replace(/<[^>]*>/g, "")).toContain("Часов доступно: 48 из 48");
	expect(html).toContain("0.0123%");
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
	expect(text).toContain("Дней доступно: 0 из 30");
	expect(text).toContain("Hourly Range—");
	expect(text).toContain("Часов доступно: —");
});
