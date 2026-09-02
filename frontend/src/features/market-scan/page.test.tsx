import { MantineProvider } from "@mantine/core";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { marketScanQueryOptions } from "@/api/market-scan";
import type { MarketScanResult } from "../../api/client";
import { BusinessRequestContext } from "../../app/business-request-context";
import { MarketScanPage } from "./page";
import {
	criterionSelections,
	defaultMarketScanCriteria,
	type MarketScanCriteria,
} from "./pipeline";
import { defaultMarketScanSort, type MarketScanSort } from "./sort";

function renderScan(
	criteria: MarketScanCriteria,
	result?: MarketScanResult,
	sort = defaultMarketScanSort,
) {
	const client = new QueryClient();
	if (result) {
		client.setQueryData(
			marketScanQueryOptions(criterionSelections(criteria)).queryKey,
			result,
		);
	}
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
						onSortChange={async () => {}}
						sort={sort}
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

it.each<MarketScanSort>([
	{ column: "daily_volatility", direction: "desc" },
	{ column: "daily_volatility", direction: "asc" },
	{ column: "hourly_volatility", direction: "desc" },
	{ column: "hourly_volatility", direction: "asc" },
	{ column: "market_cap", direction: "desc" },
	{ column: "market_cap", direction: "asc" },
])("renders %s sorting direction in the active header", (sort) => {
	const html = renderScan(
		defaultMarketScanCriteria,
		{
			analyzed_count: 1,
			insufficient_data_count: 0,
			items: [
				{
					evaluations: [
						{
							candle_count: 30,
							from: "2026-08-01T00:00:00Z",
							key: "daily_volatility",
							label: "Daily Volatility",
							matched: true,
							metrics: { range_percent: 6 },
							name: "volatility",
							to: "2026-08-02T00:00:00Z",
						},
						{
							candle_count: 60,
							from: "2026-08-01T00:00:00Z",
							key: "hourly_volatility",
							label: "Hourly Volatility",
							matched: true,
							metrics: { range_percent: 2 },
							name: "volatility",
							to: "2026-08-02T00:00:00Z",
						},
						{
							candle_count: 0,
							from: "0001-01-01T00:00:00Z",
							key: "market_cap",
							label: "Market Cap",
							matched: true,
							metrics: { market_cap_usd: 500_000_000 },
							name: "market_cap",
							to: "0001-01-01T00:00:00Z",
						},
					],
					matched: true,
					symbol: "BTCUSDT",
				},
			],
			matched_count: 1,
			unresolved: [],
			warnings: [],
		},
		sort,
	);
	expect(html).toContain(
		`aria-sort="${sort.direction === "desc" ? "descending" : "ascending"}"`,
	);
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
		/Daily Range.*Daily Candle Count.*Hourly Range.*Hourly Candle Count/,
	);
	expect(html).toContain('aria-sort="descending"');
	expect(html).toContain(
		'aria-label="Sort by Daily Range, currently descending"',
	);
	expect(html).toContain('aria-label="Sort by Hourly Range"');
	expect(html).toMatch(/6.25%<.*30<.*2.75%<.*60</);
	expect(html).toContain("Market Cap USD");
	expect(html).toContain('aria-label="Sort by Market Cap USD"');
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
		marketScanQueryOptions(
			criterionSelections({ ...defaultMarketScanCriteria, [field]: 10 }),
		).queryKey,
	).not.toEqual(
		marketScanQueryOptions(criterionSelections(defaultMarketScanCriteria))
			.queryKey,
	);
});
