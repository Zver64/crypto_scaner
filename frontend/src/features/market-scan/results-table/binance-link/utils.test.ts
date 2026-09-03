import { describe, expect, it } from "vitest";
import { binanceSpotUrl } from "@/features/market-scan/results-table/binance-link/utils";

describe("binanceSpotUrl", () => {
	it("builds a Binance Spot trading URL for a USDT symbol", () => {
		expect(binanceSpotUrl("BTCUSDT")).toBe(
			"https://www.binance.com/en/trade/BTC_USDT?type=spot",
		);
	});

	it("normalizes surrounding whitespace and symbol casing", () => {
		expect(binanceSpotUrl(" adausdt ")).toBe(
			"https://www.binance.com/en/trade/ADA_USDT?type=spot",
		);
	});

	it.each(["", "USDT", "BTCEUR"])("rejects unsupported symbol %j", (symbol) => {
		expect(binanceSpotUrl(symbol)).toBeUndefined();
	});
});
