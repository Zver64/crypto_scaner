import { describe, expect, it } from "vitest";
import type { MarketScanItem } from "@/api/client";
import {
	topCoinsCriteria,
	topCoinsRequestOptions,
	toTopCoinRows,
} from "@/features/top-coins/top-coins";

function item(symbol: string): MarketScanItem {
	return {
		evaluations: [],
		matched: true,
		price_history: [],
		symbol,
	};
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

describe("topCoinsRequestOptions", () => {
	it("asks the backend for the ten largest market caps", () => {
		expect(topCoinsRequestOptions).toEqual({
			limit: 10,
			sort: { direction: "desc", field: "market_cap_usd" },
		});
	});
});

describe("toTopCoinRows", () => {
	it("preserves the backend result without client-side ranking or limiting", () => {
		const items = Array.from({ length: 11 }, (_, index) =>
			item(`COIN${index}`),
		);

		expect(toTopCoinRows(items).map(({ symbol }) => symbol)).toEqual(
			items.map(({ symbol }) => symbol),
		);
	});
});
