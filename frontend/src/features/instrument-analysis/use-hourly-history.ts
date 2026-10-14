import type { InfiniteData } from "@tanstack/react-query";

import {
	listInstrumentCandles,
	type listInstrumentCandlesResponseSuccess,
	useListInstrumentCandlesInfinite,
} from "@/api/generated/api";
import type {
	Candle,
	ListInstrumentCandlesParams,
} from "@/api/generated/models";
import {
	nextCandlePageParam,
	validateCandlePage,
} from "@/features/instrument-analysis/candle-page";

// The 7-day window moves with the clock, so the history refreshes hourly.
const refreshMilliseconds = 60 * 60 * 1_000;

const params: ListInstrumentCandlesParams = { interval: "1h", limit: 200 };

// Joins the pages oldest first; the result is structurally shared, so it keeps
// its reference while the history does not change.
function oldestFirstCandles(
	history: InfiniteData<listInstrumentCandlesResponseSuccess, unknown>,
): Candle[] {
	return [...history.pages].reverse().flatMap((page) => page.data.candles);
}

// Hourly candles for the page's 7d change and spot grid, not for the chart.
export function useHourlyHistory(symbol: string, enabled: boolean) {
	const query = useListInstrumentCandlesInfinite<Candle[]>(symbol, params, {
		query: {
			enabled,
			getNextPageParam: nextCandlePageParam,
			initialPageParam: undefined,
			meta: {
				errorNotification: {
					id: `hourly-price-history-${symbol}-error`,
					title: "Hourly price history failed",
				},
			},
			// Validated when fetched, so an unexpected page fails the query and
			// reaches the query cache error handler.
			queryFn: async ({ pageParam, signal }) => {
				const response = await listInstrumentCandles(
					symbol,
					{ ...params, before: pageParam },
					{ signal },
				);
				validateCandlePage(response, symbol, params.interval);
				return response;
			},
			refetchInterval: refreshMilliseconds,
			retry: false,
			staleTime: refreshMilliseconds,
			select: oldestFirstCandles,
		},
	});
	return { candles: query.data ?? [], pending: query.isPending };
}
