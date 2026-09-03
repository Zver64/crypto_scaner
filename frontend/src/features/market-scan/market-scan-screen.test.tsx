import { MantineProvider } from "@mantine/core";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import type { MarketScanResult } from "@/api/client";
import { marketScanQueryOptions } from "@/api/market-scan";
import { BusinessRequestContext } from "@/app/business-request-context";
import { MarketScanScreen } from "@/features/market-scan/market-scan-screen";
import {
	criterionSelections,
	defaultMarketScanCriteria,
	type MarketScanCriteria,
} from "@/features/market-scan/pipeline";
import { marketScanColumnKeys } from "@/features/market-scan/results-table/keys";
import {
	defaultMarketScanSort,
	type MarketScanSort,
} from "@/features/market-scan/sort";

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
					<MarketScanScreen
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
	expect(html).toContain('data-size="h3"');
	expect(html).toContain('data-size="xs"');
	expect(html).toContain("Hourly Volatility");
	expect(html).toContain("Market Cap");
	expect(html).toContain('value="30"');
	expect(html).toContain('value="60"');
	expect(html).toContain(
		`value="${defaultMarketScanCriteria.minimumRangePercent}"`,
	);
	expect(html).toContain('value="2"');
	expect(html).toContain(
		`value="${defaultMarketScanCriteria.minimumMarketCapMillions}"`,
	);
	expect(html).toContain("Minimum Range (%)");
	expect(html).toContain("Minimum Market Cap (USD millions)");
	expect(html).not.toContain('type="radio"');
	expect(html).toContain("Run Market Scan");
});

it.each<MarketScanSort>([
	{ column: marketScanColumnKeys.dailyRange, direction: "desc" },
	{ column: marketScanColumnKeys.dailyRange, direction: "asc" },
	{ column: marketScanColumnKeys.hourlyRange, direction: "desc" },
	{ column: marketScanColumnKeys.hourlyRange, direction: "asc" },
	{ column: marketScanColumnKeys.marketCap, direction: "desc" },
	{ column: marketScanColumnKeys.marketCap, direction: "asc" },
])("renders %s sorting direction in the active header", (sort) => {
	const html = renderScan(
		defaultMarketScanCriteria,
		{
			analyzed_count: 1,
			price_history_window: {
				from: "2026-08-26T23:00:00Z",
				to: "2026-09-02T23:00:00Z",
			},
			insufficient_data_count: 0,
			items: [
				{
					price_history: Array(169).fill(null),
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

it("identifies metrics by key and keeps every column when the Market Cap criterion is disabled", () => {
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
		price_history_window: {
			from: "2026-08-26T23:00:00Z",
			to: "2026-09-02T23:00:00Z",
		},
		analyzed_count: 1,
		matched_count: 1,
		insufficient_data_count: 0,
		items: [
			{
				price_history: Array.from({ length: 169 }, (_, i) => i + 1),
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
		/Daily Range.*Hourly Range.*Hourly Candle Count.*Daily Candle Count.*Market Cap USD.*7d change.*Binance/,
	);
	const sortButtons = html.match(
		/<button[^>]*aria-label="Sort by [^"]*"[^>]*>/g,
	);
	expect(sortButtons).toHaveLength(3);
	for (const button of sortButtons ?? []) {
		expect(button).toContain("font:inherit");
	}
	expect(html).toContain("BTCUSDT: 7-day hourly closing prices");
	expect(html).toContain('aria-sort="descending"');
	expect(html).toContain(
		'aria-label="Sort by Daily Range, currently descending"',
	);
	expect(html).toContain('aria-label="Sort by Hourly Range"');
	expect(html).toMatch(/6.25%<.*2.75%<.*60<.*30<.*\$750M/);
	expect(html).toContain("Market Cap USD");
	expect(html).toContain('aria-label="Sort by Market Cap USD"');
	expect(html).toContain("$750M");
	expect(html).toContain('aria-label="Open BTCUSDT on Binance Spot"');
	expect(html).toContain('style="text-align:center"');
	expect(html).toContain('data-icon="external-link"');
	expect(html).toContain('height="16"');
	expect(html).toContain('width="16"');
	expect(html).toContain("font-size:var(--mantine-font-size-xs)");
	expect(html).not.toContain(">Open ↗</a>");
	expect(html).toContain("https://www.binance.com/en/trade/BTC_USDT?type=spot");
	const disabledHtml = renderScan(
		{ ...defaultMarketScanCriteria, minimumMarketCapMillions: 0 },
		result,
	);
	expect(disabledHtml).toContain("Market Cap USD");
	expect(disabledHtml).toContain("$750M");
	const withoutMarketCap = {
		...result,
		items: result.items.map((item) => ({
			...item,
			evaluations: [hourly, daily],
		})),
	};
	const noCapHtml = renderScan(
		{ ...defaultMarketScanCriteria, minimumMarketCapMillions: 0 },
		withoutMarketCap,
		{ column: marketScanColumnKeys.marketCap, direction: "desc" },
	);
	const headers = (markup: string) =>
		[...markup.matchAll(/<th\b[^>]*>([\s\S]*?)<\/th>/g)].map((match) =>
			match[1]
				.replace(/<[^>]+>/g, "")
				.replace(/[↑↓]/g, "")
				.trim(),
		);
	expect(headers(disabledHtml)).toEqual(headers(html));
	expect(headers(noCapHtml)).toEqual(headers(html));
	expect(headers(noCapHtml)).toHaveLength(8);
	expect(noCapHtml).toContain("BTCUSDT");
	expect(noCapHtml).toContain(
		'aria-label="Sort by Market Cap USD, currently descending"',
	);
	expect(noCapHtml).toMatch(/<td[^>]*>—<\/td>/);
	expect(noCapHtml).not.toContain("$750M");
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
