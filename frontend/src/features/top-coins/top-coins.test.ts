import { describe, expect, it } from "vitest";
import type { Evaluation, MarketScanItem } from "@/api/client";
import {
	topCoinsCriteria,
	toTopCoinRows,
} from "@/features/top-coins/top-coins";

function item(symbol: string, marketCapUsd?: number): MarketScanItem {
	const evaluations: Evaluation[] =
		marketCapUsd === undefined
			? []
			: [
					{
						candle_count: 0,
						from: "2026-01-01T00:00:00Z",
						key: "market_cap",
						label: "Market Cap",
						matched: true,
						metrics: { market_cap_usd: marketCapUsd },
						name: "market_cap",
						to: "2026-01-01T00:00:00Z",
					},
				];
	return {
		evaluations,
		matched: true,
		price_history: [],
		symbol,
	};
}

function ranking(items: readonly MarketScanItem[]) {
	return toTopCoinRows(items).map(({ marketCapUsd, symbol }) => ({
		marketCapUsd,
		symbol,
	}));
}

describe("topCoinsCriteria", () => {
	it("requests the same evaluations as Market Scan without filtering by them", () => {
		expect(topCoinsCriteria.map(({ key }) => key)).toEqual([
			"daily_volatility",
			"hourly_volatility",
			"market_cap",
		]);
		expect(topCoinsCriteria.map(({ parameters }) => parameters)).toEqual([
			expect.objectContaining({ minimum_range_percent: 0 }),
			expect.objectContaining({ minimum_range_percent: 0 }),
			{ min_market_cap_usd: 0 },
		]);
	});
});

describe("toTopCoinRows", () => {
	it("sorts descending, resolves ties by symbol, and limits the result", () => {
		const items = [
			item("F", 1),
			item("B", 5),
			item("A", 5),
			item("C", 4),
			item("D", 3),
			item("E", 2),
		];

		expect(ranking(items)).toEqual([
			{ marketCapUsd: 5, symbol: "A" },
			{ marketCapUsd: 5, symbol: "B" },
			{ marketCapUsd: 4, symbol: "C" },
			{ marketCapUsd: 3, symbol: "D" },
			{ marketCapUsd: 2, symbol: "E" },
		]);
		expect(items.map(({ symbol }) => symbol)).toEqual([
			"F",
			"B",
			"A",
			"C",
			"D",
			"E",
		]);
	});

	it("returns fewer than five rows and excludes missing evaluations", () => {
		expect(ranking([item("BTC", 10), item("UNKNOWN")])).toEqual([
			{ marketCapUsd: 10, symbol: "BTC" },
		]);
	});

	it("includes a zero market cap", () => {
		expect(ranking([item("ZERO", 0)])).toEqual([
			{ marketCapUsd: 0, symbol: "ZERO" },
		]);
	});

	it("excludes an evaluation whose market cap metric is missing", () => {
		const missingMetric = item("UNKNOWN", 10);
		missingMetric.evaluations[0]!.metrics = {};

		expect(ranking([item("BTC", 10), missingMetric])).toEqual([
			{ marketCapUsd: 10, symbol: "BTC" },
		]);
	});

	it("returns an empty result for empty input", () => {
		expect(ranking([])).toEqual([]);
	});
});
