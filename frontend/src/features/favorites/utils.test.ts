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
					closed_indicators: [
						{
							type: "rsi",
							interval: "1d",
							parameters: { period: 14 },
							open_time: "2026-01-01T00:00:00Z",
							outputs: [{ name: "rsi", value: 28.5 }],
						},
					],
				},
				{
					symbol: "OLDUSDT",
					base_asset: "OLD",
					quote_asset: "USDT",
					active: false,
					alert_count: 0,
					created_at: "2026-01-01T00:00:00Z",
					closed_indicators: [],
				},
			],
			[],
		);
		expect(rows.map((row) => row.symbol)).toEqual(["BTCUSDT", "OLDUSDT"]);
		// Closed indicators come with the favorite even without an analysis row.
		expect(rows[0]).toMatchObject({ dailyRsi14: 28.5, marketCapUsd: null });
		expect(rows[1]).toMatchObject({
			dailyRsi14: null,
			marketCapUsd: null,
			priceHistory: [],
		});
	});
});
