import { MantineProvider } from "@mantine/core";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { InstrumentAnalysisResult } from "@/api/client";
import { instrumentAnalysisQueryKey } from "@/api/instrument-analysis";
import { BusinessRequestContext } from "@/app/business-request-context";
import { InstrumentAnalysisScreen } from "@/features/instrument-analysis/instrument-analysis-screen";
import {
	criterionSelections,
	defaultMarketScanCriteria,
} from "@/features/market-scan/pipeline";

function renderAnalysis(result: InstrumentAnalysisResult) {
	vi.stubGlobal("window", {});
	const client = new QueryClient();
	client.setQueryData(
		instrumentAnalysisQueryKey(
			"BTCUSDT",
			criterionSelections(defaultMarketScanCriteria),
		),
		result,
	);
	return renderToStaticMarkup(
		<MantineProvider>
			<QueryClientProvider client={client}>
				<BusinessRequestContext
					value={{ allowed: true, authenticated: true, backendReady: true }}
				>
					<InstrumentAnalysisScreen
						criterionSelections={criterionSelections(defaultMarketScanCriteria)}
						onBack={() => {}}
						symbol="BTCUSDT"
					/>
				</BusinessRequestContext>
			</QueryClientProvider>
		</MantineProvider>,
	);
}

it("shows the instrument summary and market cap without volatility details", () => {
	const daily = {
		candle_count: 30,
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
		candle_count: 60,
		key: "hourly_volatility",
		label: "Hourly Volatility",
		metrics: { range_percent: 2.75 },
	};
	const html = renderAnalysis({
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
	});

	expect(html).toContain("Symbol");
	expect(html).toContain("BTCUSDT");
	expect(html).toContain("Market Cap");
	expect(html).toContain("$1.5B");
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
});
