import { MantineProvider } from "@mantine/core";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import type { MarketScanResult } from "../../api/client";
import { BusinessRequestContext } from "../../app/business-request-context";
import { MarketScanPage, marketScanQueryKey } from "./market-scan-page";
import { defaultMarketScanCriteria, type MarketScanCriteria } from "./pipeline";

function renderScan(criteria: MarketScanCriteria, result?: MarketScanResult) {
	const client = new QueryClient();
	if (result) client.setQueryData(marketScanQueryKey(criteria), result);
	return renderToStaticMarkup(
		<MantineProvider>
			<QueryClientProvider client={client}>
				<BusinessRequestContext
					value={{ allowed: true, authenticated: true, backendReady: true }}
				>
					<MarketScanPage
						committedCriteria={criteria}
						onCommit={async () => {}}
						onSelectInstrument={async () => {}}
					/>
				</BusinessRequestContext>
			</QueryClientProvider>
		</MantineProvider>,
	);
}

it("renders separate mandatory volatility controls without a mode selector", () => {
	const html = renderScan(defaultMarketScanCriteria);
	expect(html).toContain("Daily Volatility");
	expect(html).toContain("Hourly Volatility");
	expect(html).toContain("Market Cap");
	expect(html).toContain('value="30 days"');
	expect(html).toContain('value="60 hours"');
	expect(html).toContain('value="5%"');
	expect(html).toContain('value="2%"');
	expect(html).not.toContain('type="radio"');
	expect(html).toContain("Run Market Scan");
});

it("identifies both ranges and candle counts by key and hides inactive Market Cap results", () => {
	const daily = {
		key: "daily_volatility",
		name: "volatility",
		label: "Daily Volatility",
		matched: true,
		candle_count: 30,
		from: "2026-08-01T00:00:00Z",
		to: "2026-08-30T00:00:00Z",
		metrics: { range_percent: 6.25 },
	};
	const hourly = {
		...daily,
		key: "hourly_volatility",
		label: "Hourly Volatility",
		candle_count: 60,
		metrics: { range_percent: 2.75 },
	};
	const marketCap = {
		...daily,
		key: "market_cap",
		name: "market_cap",
		label: "Market Cap",
		metrics: { market_cap_usd: 750_000_000 },
	};
	const result = {
		analyzed_count: 1,
		matched_count: 1,
		insufficient_data_count: 0,
		items: [
			{
				symbol: "BTCUSDT",
				matched: true,
				evaluations: [hourly, marketCap, daily],
			},
		],
		unresolved: [],
		warnings: [],
	};
	const html = renderScan(defaultMarketScanCriteria, result);
	expect(html).toMatch(
		/Daily Range<.*Daily Candle Count<.*Hourly Range<.*Hourly Candle Count</,
	);
	expect(html).toMatch(/6.25%<.*30<.*2.75%<.*60</);
	expect(html).toContain("Market Cap USD");
	expect(html).toContain("$750M");
	expect(html).toContain("https://www.binance.com/en/trade/BTC_USDT?type=spot");
	const disabledHtml = renderScan(
		{ ...defaultMarketScanCriteria, minimumMarketCapMillions: 0 },
		result,
	);
	expect(disabledHtml).not.toContain("Market Cap USD");
	expect(disabledHtml).not.toContain("$750M");
});

it.each([
	"hourlyPeriod",
	"hourlyPercentile",
	"hourlyMinimumRangePercent",
] as const)("separates cached scans when %s changes", (field) => {
	expect(
		marketScanQueryKey({ ...defaultMarketScanCriteria, [field]: 10 }),
	).not.toEqual(marketScanQueryKey(defaultMarketScanCriteria));
});
