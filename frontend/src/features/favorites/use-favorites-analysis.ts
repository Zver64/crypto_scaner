import { notifications } from "@mantine/notifications";
import { keepPreviousData } from "@tanstack/react-query";
import { useEffect } from "react";
import {
	getAnalyzeFavoritesQueryKey,
	useAnalyzeFavorites,
} from "@/api/generated/api";
import type { MarketAnalysisResponse } from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { apiErrorMessage } from "@/features/analysis/api-error";
import { hasExpectedMarketScanResult } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import { useFavorites } from "@/features/favorites/favorites-provider";
import {
	scopedUserQueryKey,
	telegramUserScope,
} from "@/features/favorites/user-query-scope";
import {
	buildTopCoinsCriteria,
	buildTopCoinsScanCriteria,
	topCoinsRequestOptions,
} from "@/features/top-coins/top-coins";

export type FavoritesAnalysis = ReturnType<typeof useFavoritesAnalysis>;

// The favorites table under the given settings, refreshed periodically.
export function useFavoritesAnalysis(settings: VolatilitySettings) {
	const permission = useBusinessRequestPermission();
	const { favorites, handleAccessError, isError, isLoading } = useFavorites();
	const hasFavorites = favorites.size > 0;
	const scanCriteria = buildTopCoinsScanCriteria(settings);
	const criteria = buildTopCoinsCriteria(settings);
	const request = { criteria, ...topCoinsRequestOptions };
	const query = useAnalyzeFavorites<MarketAnalysisResponse>(request, {
		query: {
			enabled: permission.allowed && !isLoading && hasFavorites,
			queryKey: scopedUserQueryKey(
				getAnalyzeFavoritesQueryKey(request),
				telegramUserScope(),
			),
			// Settings changes keep the current table until the new one arrives.
			placeholderData: keepPreviousData,
			refetchInterval: 15_000,
			refetchOnMount: "always",
			retry: false,
			select: (response) => {
				if (!hasExpectedMarketScanResult(response.data, criteria)) {
					throw new Error("Unexpected favorites analysis response");
				}
				return response.data;
			},
		},
	});
	useEffect(() => {
		if (!query.isError) return;
		handleAccessError(query.error);
		notifications.show({
			id: "favorites-analysis-error",
			autoClose: 5000,
			color: "red",
			message: apiErrorMessage(query.error),
			title: "Favorites analysis failed",
		});
	}, [handleAccessError, query.error, query.isError]);
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
