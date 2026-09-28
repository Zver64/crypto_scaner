import {
	Center,
	Container,
	Loader,
	Paper,
	Stack,
	Text,
	TextInput,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useEffect, useState } from "react";
import {
	getAnalyzeFavoritesQueryKey,
	useAnalyzeFavorites,
} from "@/api/generated/api";
import type { MarketAnalysisResponse } from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { telegramRequestOptions } from "@/app/telegram";
import { SettingsForm } from "@/components/settings-form";
import { apiErrorMessage } from "@/features/analysis/api-error";
import { hasExpectedMarketScanResult } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import { useVolatilitySettingsForm } from "@/features/analysis/volatility-settings-form/use-volatility-settings-form";
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
import { filterMarketScanRows } from "@/features/market-scan/results-table/utils";
import type { MarketScanSort } from "@/features/market-scan/sort";
import {
	buildTopCoinsCriteria,
	buildTopCoinsScanCriteria,
	topCoinsRequestOptions,
} from "@/features/top-coins/top-coins";

export function FavoritesScreen({
	initialSettings,
	onSettingsCommit,
	onSortChange,
	onSymbolFilterChange,
	sort,
	symbolFilter,
}: {
	initialSettings: VolatilitySettings;
	onSettingsCommit(settings: VolatilitySettings): void;
	onSortChange(sort: MarketScanSort): void;
	onSymbolFilterChange(symbolFilter: string): void;
	sort: MarketScanSort;
	symbolFilter: string;
}) {
	const permission = useBusinessRequestPermission();
	const { favorites, handleAccessError, isError, isLoading } = useFavorites();
	const favoriteItems = [...favorites.values()];
	const [settings, setSettings] = useState(initialSettings);
	const settingsForm = useVolatilitySettingsForm({
		disabled: !permission.allowed,
		initialSettings,
		onCommit: (nextSettings) => {
			setSettings(nextSettings);
			onSettingsCommit(nextSettings);
		},
	});
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
	const allRows = mergeFavoriteRows(favoriteItems, query.data?.items ?? []);
	const rows = filterMarketScanRows(allRows, symbolFilter);

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap="md">
				<PageNavigation current="favorites" title="Favorites" />
				<SettingsForm {...settingsForm} />
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
				{allRows.length > 0 ? (
					<>
						<TextInput
							aria-label="Filter Favorites by symbol"
							label="Symbol filter"
							labelProps={{ mb: "xs" }}
							onChange={(event) =>
								onSymbolFilterChange(event.currentTarget.value)
							}
							placeholder="e.g. BTC"
							size="md"
							value={symbolFilter}
						/>
						{rows.length > 0 ? (
							<MarketScanResultsTable
								alertCounts={favoriteAlertCounts(favoriteItems)}
								criteria={scanCriteria}
								onSortChange={onSortChange}
								rows={rows}
								sort={sort}
								window={query.data?.price_history_window}
							/>
						) : (
							<Paper p="xl" ta="center">
								<Text fw={600}>No instruments match this symbol filter.</Text>
								<Text c="dimmed" mt={4} size="sm">
									Clear or change the filter to see all instruments.
								</Text>
							</Paper>
						)}
					</>
				) : null}
			</Stack>
		</Container>
	);
}
