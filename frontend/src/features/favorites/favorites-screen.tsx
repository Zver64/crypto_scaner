import { Center, Container, Loader, Paper, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useEffect } from "react";
import {
	getAnalyzeFavoritesQueryKey,
	useAnalyzeFavorites,
} from "@/api/generated/api";
import type { MarketAnalysisResponse } from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { telegramRequestOptions } from "@/app/telegram";
import { applicationConfig } from "@/config";
import { apiErrorMessage } from "@/features/analysis/api-error";
import { hasExpectedMarketScanResult } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { useFavorites } from "@/features/favorites/favorites-provider";
import {
	scopedUserQueryKey,
	telegramUserScope,
} from "@/features/favorites/user-query-scope";
import {
	favoriteAlertCounts,
	mergeFavoriteRows,
} from "@/features/favorites/utils";
import { MarketScanResultsTable } from "@/features/market-scan/results-table";
import type { MarketScanSort } from "@/features/market-scan/sort";
import {
	buildTopCoinsCriteria,
	buildTopCoinsScanCriteria,
	topCoinsRequestOptions,
} from "@/features/top-coins/top-coins";

export function FavoritesScreen({
	onSortChange,
	sort,
}: {
	onSortChange(sort: MarketScanSort): void;
	sort: MarketScanSort;
}) {
	const permission = useBusinessRequestPermission();
	const { favorites, handleAccessError, isError, isLoading } = useFavorites();
	const favoriteItems = [...favorites.values()];
	const settings = applicationConfig.topMarketCap.defaultSettings;
	const scanCriteria = buildTopCoinsScanCriteria(settings);
	const criteria = buildTopCoinsCriteria(settings);
	const request = { criteria, ...topCoinsRequestOptions };
	const query = useAnalyzeFavorites<MarketAnalysisResponse>(request, {
		fetch: telegramRequestOptions(),
		query: {
			enabled: permission.allowed && !isLoading && favoriteItems.length > 0,
			queryKey: scopedUserQueryKey(
				getAnalyzeFavoritesQueryKey(request),
				telegramUserScope(),
			),
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
	const rows = mergeFavoriteRows(favoriteItems, query.data?.items ?? []);

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap="md">
				<PageNavigation current="favorites" title="Favorites" />
				{isLoading || (query.isPending && favoriteItems.length > 0) ? (
					<Center mih={180}>
						<Loader aria-label="Loading favorites" />
					</Center>
				) : null}
				{isError && favoriteItems.length === 0 ? (
					<Paper p="xl" ta="center">
						<Text fw={600}>Unable to load favorites.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Try again when access and the backend are available.
						</Text>
					</Paper>
				) : null}
				{!isLoading && !isError && favoriteItems.length === 0 ? (
					<Paper p="xl" ta="center">
						<Text fw={600}>No favorites yet.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Use the star in a market table to add one.
						</Text>
					</Paper>
				) : null}
				{rows.length > 0 ? (
					<MarketScanResultsTable
						alertCounts={favoriteAlertCounts(favoriteItems)}
						criteria={scanCriteria}
						onSortChange={onSortChange}
						rows={rows}
						sort={sort}
						window={query.data?.price_history_window}
					/>
				) : null}
			</Stack>
		</Container>
	);
}
