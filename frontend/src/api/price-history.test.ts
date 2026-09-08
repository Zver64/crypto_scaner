import { expect, it } from "vitest";
import { fetchInstrumentAnalysis, fetchMarketScan } from "@/api/client";

const window = { from: "2026-08-26T23:00:00Z", to: "2026-09-02T23:00:00Z" };
const instrumentWindow = {
	from: "2026-08-03T23:00:00Z",
	to: "2026-09-02T23:00:00Z",
};
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

function candle(slot: number, close: number) {
	return {
		close,
		high: close + 1,
		low: close - 1,
		open: close - 0.5,
		open_time: new Date(
			Date.parse(instrumentWindow.from) + slot * 3_600_000,
		).toISOString(),
	};
}

const candles = Array(721).fill(null);
candles[648] = candle(648, 1.00000001);
candles[720] = candle(720, 0.99999999);
const instrumentPayload = {
	candle_history: candles,
	evaluations: [],
	matched: true,
	price_history_window: instrumentWindow,
	symbol: "BTCUSDT",
	warnings: [],
};

it("preserves the market window, missing positions and unrounded prices", async () => {
	const result = await fetchMarketScan([], {
		request: async () => Response.json(payload),
	});
	expect(result).toEqual(payload);
});

it("preserves the instrument OHLC history and UTC slots", async () => {
	await expect(
		fetchInstrumentAnalysis("BTCUSDT", [], {
			request: async () => Response.json(instrumentPayload),
		}),
	).resolves.toEqual(instrumentPayload);
});

it.each([
	{ ...instrumentPayload, candle_history: [null] },
	{
		...instrumentPayload,
		candle_history: candles.map((value, index) =>
			index === 648 ? { ...value, open_time: instrumentWindow.from } : value,
		),
	},
	{
		...instrumentPayload,
		candle_history: candles.map((value, index) =>
			index === 648 ? { ...value, high: Number.NaN } : value,
		),
	},
	{
		...instrumentPayload,
		candle_history: candles.map((value, index) =>
			index === 648 ? { ...value, low: 2 } : value,
		),
	},
	{
		...instrumentPayload,
		price_history_window: {
			from: "2026-08-02T23:00:00Z",
			to: instrumentWindow.to,
		},
	},
	{ ...instrumentPayload, price_history_window: undefined },
])("rejects malformed instrument candle history", async (body) => {
	await expect(
		fetchInstrumentAnalysis("BTCUSDT", [], {
			request: async () => Response.json(body),
		}),
	).rejects.toMatchObject({ code: "unexpected_error" });
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
])("rejects malformed market history instead of stretching or coercing prices", async (body) => {
	await expect(
		fetchMarketScan([], { request: async () => Response.json(body) }),
	).rejects.toMatchObject({ code: "unexpected_error" });
});
