import { keepPreviousData } from "@tanstack/react-query";

import {
	analyzeFavorites,
	type analyzeFavoritesResponseSuccess,
	getAnalyzeFavoritesQueryKey,
	useAnalyzeFavorites,
} from "@/api/generated/api";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { unexpectedApiError } from "@/features/analysis/api-error";
import { hasExpectedMarketScanResult } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import { useFavorites } from "@/features/favorites/favorites-context";
import {
	scopedUserQueryKey,
	telegramUserScope,
} from "@/features/favorites/user-query-scope";
import { unfilteredScanCriteria } from "@/features/market-scan/pipeline";
import {
	buildTopCoinsCriteria,
	topCoinsRequestOptions,
} from "@/features/top-coins/top-coins";

const selectAnalysis = (response: analyzeFavoritesResponseSuccess) =>
	response.data;

export type FavoritesAnalysis = ReturnType<typeof useFavoritesAnalysis>;

// The favorites table under the given settings, refreshed periodically.
export function useFavoritesAnalysis(settings: VolatilitySettings) {
	const permission = useBusinessRequestPermission();
	const { favorites, isError, isLoading } = useFavorites();
	const hasFavorites = favorites.size > 0;
	const scanCriteria = unfilteredScanCriteria(settings);
	const criteria = buildTopCoinsCriteria(settings);
	const request = { criteria, ...topCoinsRequestOptions };
	const query = useAnalyzeFavorites(request, {
		query: {
			enabled: permission.allowed && !isLoading && hasFavorites,
			meta: {
				errorNotification: {
					id: "favorites-analysis-error",
					title: "Favorites analysis failed",
				},
				userScoped: true,
			},
			queryKey: scopedUserQueryKey(
				getAnalyzeFavoritesQueryKey(request),
				telegramUserScope(),
			),
			// Settings changes keep the current table until the new one arrives.
			placeholderData: keepPreviousData,
			refetchInterval: 15_000,
			refetchOnMount: "always",
			// Validated when fetched, so an unexpected response fails the query
			// and reaches the query cache error handler.
			queryFn: async ({ signal }) => {
				const response = await analyzeFavorites(request, { signal });
				if (!hasExpectedMarketScanResult(response.data, criteria)) {
					throw unexpectedApiError();
				}
				return response;
			},
			retry: false,
			select: selectAnalysis,
		},
	});
	useAnalysisWarningNotification(query.data?.warnings, "Favorites warning");
	// The analysis table has a row for every favorite, including instruments
	// the analysis skipped. Removed favorites disappear before the refetch.
	const table = hasFavorites ? query.data?.table : undefined;
	return {
		allRows: table?.rows.filter((row) => favorites.has(row.symbol)) ?? [],
		hasFavorites,
		isError,
		isLoading,
		query,
		scanCriteria,
		table,
	};
}
