import { describe, expect, it } from "vitest";
import type {
	TableRow,
	UnresolvedInstrumentCode,
} from "@/api/generated/models";
import {
	filterMarketScanRows,
	marketCapUnavailableReason,
} from "@/features/market-scan/results-table/utils";

const items: TableRow[] = [
	{ symbol: "ZZZUSDT", cells: {} },
	{ symbol: "AdaUsdt", cells: {} },
	{ symbol: "AAAUSDT", cells: {} },
];

describe("Market Cap presentation", () => {
	it.each<[UnresolvedInstrumentCode, string]>([
		[
			"mapping_not_found",
			"No market capitalization match was found for this instrument.",
		],
		[
			"mapping_conflict",
			"Multiple market capitalization matches were found for this instrument.",
		],
		[
			"mapping_provider_unavailable",
			"Market capitalization mapping data is temporarily unavailable.",
		],
		[
			"market_cap_missing",
			"Market capitalization data is unavailable for this instrument.",
		],
	])("explains unresolved code %s", (code, expected) => {
		expect(marketCapUnavailableReason(code)).toBe(expected);
	});
});

describe("filterMarketScanRows", () => {
	it("returns the complete backend-ordered result for an empty filter", () => {
		expect(filterMarketScanRows(items, "  ")).toEqual(items);
	});

	it("matches partial symbols without case sensitivity", () => {
		expect(filterMarketScanRows(items, "usdt")).toEqual(items);
		expect(filterMarketScanRows(items, "ada")).toEqual([items[1]]);
	});

	it("returns no rows for a missing symbol", () => {
		expect(filterMarketScanRows(items, "BTC")).toEqual([]);
	});

	it("preserves exact symbols, values, object identity, and backend order", () => {
		const filtered = filterMarketScanRows(items, "a");

		expect(filtered).toEqual([items[1], items[2]]);
		expect(filtered[0]).toBe(items[1]);
		expect(items.map((row) => row.symbol)).toEqual([
			"ZZZUSDT",
			"AdaUsdt",
			"AAAUSDT",
		]);
	});
});
