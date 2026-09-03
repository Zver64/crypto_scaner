import { describe, expect, it } from "vitest";
import { marketScanColumnKeys } from "@/features/market-scan/results-table/keys";
import { toMarketScanRows } from "@/features/market-scan/results-table/utils";
import {
	defaultMarketScanSort,
	nextMarketScanSort,
	sortMarketScanRows,
} from "@/features/market-scan/sort";

describe("Market Scan sorting", () => {
	it("starts a new column descending and toggles the active column", () => {
		expect(
			nextMarketScanSort(
				defaultMarketScanSort,
				marketScanColumnKeys.hourlyRange,
			),
		).toEqual({
			column: marketScanColumnKeys.hourlyRange,
			direction: "desc",
		});
		expect(
			nextMarketScanSort(defaultMarketScanSort, marketScanColumnKeys.marketCap),
		).toEqual({
			column: marketScanColumnKeys.marketCap,
			direction: "asc",
		});
	});
});

describe("sortMarketScanRows", () => {
	const daily = {
		candle_count: 30,
		from: "2026-08-01T00:00:00Z",
		key: "daily_volatility",
		label: "Daily Volatility",
		matched: true,
		metrics: { range_percent: 4 },
		name: "volatility",
		to: "2026-08-02T00:00:00Z",
	};
	const hourly = {
		...daily,
		candle_count: 60,
		key: "hourly_volatility",
		label: "Hourly Volatility",
		metrics: { range_percent: 2 },
	};
	const marketCap = {
		...daily,
		candle_count: 0,
		key: "market_cap",
		label: "Market Cap",
		metrics: { market_cap_usd: 400 },
		name: "market_cap",
	};
	const sortableItems = [
		{
			evaluations: [daily, hourly, marketCap],
			matched: true,
			symbol: "ZEBRAUSDT",
			price_history: Array(169).fill(null),
		},
		{
			evaluations: [
				{ ...daily, metrics: { range_percent: 6 } },
				{ ...hourly, metrics: { range_percent: 1 } },
				{ ...marketCap, metrics: { market_cap_usd: 900 } },
			],
			matched: true,
			symbol: "ALPHAUSDT",
			price_history: Array(169).fill(null),
		},
		{
			evaluations: [
				{ ...daily, metrics: { range_percent: 6 } },
				{ ...hourly, metrics: { range_percent: 3 } },
				{ ...marketCap, metrics: { market_cap_usd: 100 } },
			],
			matched: true,
			symbol: "BRAVOUSDT",
			price_history: Array(169).fill(null),
		},
		{
			evaluations: [
				{ ...daily, metrics: { range_percent: 5 } },
				{ ...hourly, metrics: { range_percent: 3 } },
				{ ...marketCap, metrics: { market_cap_usd: 100 } },
			],
			matched: true,
			symbol: "CHARLIEUSDT",
			price_history: Array(169).fill(null),
		},
	];

	it.each([
		[
			marketScanColumnKeys.dailyRange,
			"desc",
			["ALPHAUSDT", "BRAVOUSDT", "CHARLIEUSDT", "ZEBRAUSDT"],
		],
		[
			marketScanColumnKeys.dailyRange,
			"asc",
			["ZEBRAUSDT", "CHARLIEUSDT", "ALPHAUSDT", "BRAVOUSDT"],
		],
		[
			marketScanColumnKeys.hourlyRange,
			"desc",
			["BRAVOUSDT", "CHARLIEUSDT", "ZEBRAUSDT", "ALPHAUSDT"],
		],
		[
			marketScanColumnKeys.hourlyRange,
			"asc",
			["ALPHAUSDT", "ZEBRAUSDT", "BRAVOUSDT", "CHARLIEUSDT"],
		],
		[
			defaultMarketScanSort.column,
			defaultMarketScanSort.direction,
			["ALPHAUSDT", "ZEBRAUSDT", "BRAVOUSDT", "CHARLIEUSDT"],
		],
		[
			marketScanColumnKeys.marketCap,
			"asc",
			["BRAVOUSDT", "CHARLIEUSDT", "ZEBRAUSDT", "ALPHAUSDT"],
		],
	] as const)("sorts %s %s with alphabetical ties", (column, direction, symbols) => {
		expect(
			sortMarketScanRows(toMarketScanRows(sortableItems), {
				column,
				direction,
			}).map(({ symbol }) => symbol),
		).toEqual(symbols);
	});
});

it.each([
	"asc",
	"desc",
] as const)("sorts unavailable Market Cap last (%s), separately from zero", (direction) => {
	const base = toMarketScanRows([
		{ symbol: "BASEUSDT", evaluations: [], matched: true, price_history: [] },
	])[0];
	const rows = [
		{ ...base, symbol: "B", marketCapUsd: null },
		{ ...base, symbol: "ZERO", marketCapUsd: 0 },
		{ ...base, symbol: "A", marketCapUsd: null },
		{ ...base, symbol: "VALUE", marketCapUsd: 100 },
	];
	expect(
		sortMarketScanRows(rows, {
			column: marketScanColumnKeys.marketCap,
			direction,
		}).map((row) => row.symbol),
	).toEqual(
		direction === "asc"
			? ["ZERO", "VALUE", "A", "B"]
			: ["VALUE", "ZERO", "A", "B"],
	);
	expect(rows.map((row) => row.symbol)).toEqual(["B", "ZERO", "A", "VALUE"]);
});
