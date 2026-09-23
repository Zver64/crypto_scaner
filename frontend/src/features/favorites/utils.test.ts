import { describe, expect, it } from "vitest";
import { mergeFavoriteRows } from "@/features/favorites/utils";

describe("mergeFavoriteRows", () => {
	it("keeps favorites missing from analysis with unavailable metrics", () => {
		const rows = mergeFavoriteRows(
			[
				{
					symbol: "BTCUSDT",
					base_asset: "BTC",
					quote_asset: "USDT",
					active: true,
					alert_count: 1,
					created_at: "2026-01-01T00:00:00Z",
				},
				{
					symbol: "OLDUSDT",
					base_asset: "OLD",
					quote_asset: "USDT",
					active: false,
					alert_count: 0,
					created_at: "2026-01-01T00:00:00Z",
				},
			],
			[],
		);
		expect(rows.map((row) => row.symbol)).toEqual(["BTCUSDT", "OLDUSDT"]);
		expect(rows[1]).toMatchObject({ marketCapUsd: null, priceHistory: [] });
	});
});
