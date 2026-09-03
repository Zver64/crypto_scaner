import { expect, it } from "vitest";
import { fetchMarketScan } from "@/api/client";

const window = { from: "2026-08-26T23:00:00Z", to: "2026-09-02T23:00:00Z" };
const prices: (number | null)[] = Array.from({ length: 169 }, () => null);
prices[96] = 1.00000001;
prices[168] = 0.99999999;
const payload = {
	analyzed_count: 1,
	insufficient_data_count: 0,
	matched_count: 1,
	items: [
		{
			symbol: "BTCUSDT",
			matched: true,
			evaluations: [],
			price_history: prices,
		},
	],
	price_history_window: window,
	unresolved: [],
	warnings: [],
};

it("preserves the analysis window, missing positions and unrounded prices", async () => {
	const result = await fetchMarketScan([], {
		request: async () => Response.json(payload),
	});
	expect(result).toEqual(payload);
});

it.each([
	{
		...payload,
		price_history_window: { ...window, to: "2026-09-03T00:00:00Z" },
	},
	{
		...payload,
		price_history_window: {
			from: "2026-08-26T23:30:00Z",
			to: "2026-09-02T23:30:00Z",
		},
	},
	{ ...payload, price_history_window: undefined },
	{ ...payload, items: [{ ...payload.items[0], price_history: [1, null, 2] }] },
	{
		...payload,
		items: [{ ...payload.items[0], price_history: [...prices.slice(1), "1"] }],
	},
])("rejects malformed history instead of stretching or coercing prices", async (body) => {
	await expect(
		fetchMarketScan([], { request: async () => Response.json(body) }),
	).rejects.toMatchObject({ code: "unexpected_error" });
});
