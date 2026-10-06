import { notifications } from "@mantine/notifications";
import type { InfiniteData } from "@tanstack/react-query";
import { useEffect, useMemo } from "react";
import { useListInstrumentCandlesInfinite } from "@/api/generated/api";
import type { CandlePageResponse } from "@/api/generated/models";
import { apiErrorMessage } from "@/features/analysis/api-error";
import {
	nextCandlePageParam,
	validateCandlePage,
} from "@/features/instrument-analysis/candle-page";

// The 7-day window moves with the clock, so the history refreshes hourly.
const refreshMilliseconds = 60 * 60 * 1_000;

// Hourly candles for the page's 7d change and spot grid, not for the chart.
export function useHourlyHistory(symbol: string, enabled: boolean) {
	const query = useListInstrumentCandlesInfinite<
		InfiniteData<CandlePageResponse, string | undefined>
	>(
		symbol,
		{ interval: "1h", limit: 200 },
		{
			query: {
				enabled,
				getNextPageParam: nextCandlePageParam,
				initialPageParam: undefined,
				refetchInterval: refreshMilliseconds,
				retry: false,
				staleTime: refreshMilliseconds,
				select: (history) => ({
					...history,
					pages: history.pages.map((page) =>
						validateCandlePage(page, symbol, "1h"),
					),
				}),
			},
		},
	);
	useEffect(() => {
		if (query.isError) {
			notifications.show({
				id: `hourly-price-history-${symbol}-error`,
				autoClose: 5000,
				color: "red",
				message: apiErrorMessage(query.error),
				title: "Hourly price history failed",
			});
		}
	}, [query.error, query.isError, symbol]);
	const candles = useMemo(
		() =>
			query.data
				? [...query.data.pages].reverse().flatMap((page) => page.candles)
				: [],
		[query.data],
	);
	return { candles, pending: query.isPending };
}
