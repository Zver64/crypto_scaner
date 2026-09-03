import { expect, it } from "vitest";
import { formatMarketCapUsd, marketCapEvaluation } from "@/utils/market-cap";

it("formats compact USD values", () => {
	expect(formatMarketCapUsd(1_234_567)).toBe("$1.2M");
});

it("extracts a Market Cap evaluation without candle coverage", () => {
	expect(
		marketCapEvaluation([
			{
				candle_count: 0,
				from: "0001-01-01T00:00:00Z",
				key: "market_cap",
				label: "Market Cap",
				matched: true,
				metrics: { market_cap_usd: 1_234_567 },
				name: "market_cap",
				to: "0001-01-01T00:00:00Z",
			},
		]),
	).toEqual({ marketCapUsd: 1_234_567, matched: true });
});
