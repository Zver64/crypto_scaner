import { MantineProvider } from "@mantine/core";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import type { InstrumentAnalysisResult } from "../../api/client";
import { BusinessRequestContext } from "../../app/business-request-context";
import { defaultMarketScanCriteria } from "../market-scan/pipeline";
import {
	InstrumentAnalysisPage,
	instrumentAnalysisQueryKey,
} from "./instrument-analysis-page";

function renderAnalysis(result: InstrumentAnalysisResult) {
	vi.stubGlobal("window", {});
	const client = new QueryClient();
	client.setQueryData(
		instrumentAnalysisQueryKey("BTCUSDT", defaultMarketScanCriteria),
		result,
	);
	return renderToStaticMarkup(
		<MantineProvider>
			<QueryClientProvider client={client}>
				<BusinessRequestContext
					value={{ allowed: true, authenticated: true, backendReady: true }}
				>
					<InstrumentAnalysisPage
						committedCriteria={defaultMarketScanCriteria}
						onBack={() => {}}
						onCommit={async () => {}}
						symbol="BTCUSDT"
					/>
				</BusinessRequestContext>
			</QueryClientProvider>
		</MantineProvider>,
	);
}

it("presents independent daily and hourly evaluations by criterion-instance key", () => {
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
		evaluations: [hourly, daily],
		matched: true,
		symbol: "BTCUSDT",
		warnings: [],
	});

	expect(html).toContain("Daily Volatility");
	expect(html).toContain("Hourly Volatility");
	expect(html).toMatch(/Daily Volatility.*6.25%.*30.*Aug 1, 2026 UTC/);
	expect(html).toMatch(/Hourly Volatility.*2.75%.*60.*Aug 1, 2026 UTC/);
	expect(html).not.toContain('type="radio"');
});

it("explains that Hourly Volatility was short-circuited after a daily rejection", () => {
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

	expect(html).toContain("Hourly Volatility");
	expect(html).toContain(
		"Not evaluated because Daily Volatility did not match.",
	);
	expect(html).toContain("Market Cap");
});
