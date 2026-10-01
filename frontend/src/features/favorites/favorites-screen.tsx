import {
	Button,
	Center,
	Container,
	Loader,
	Paper,
	Stack,
	Text,
	TextInput,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { keepPreviousData } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
	getAnalyzeFavoritesQueryKey,
	useAnalyzeFavorites,
} from "@/api/generated/api";
import type { MarketAnalysisResponse } from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
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
	sort: MarketScanSort | undefined;
	symbolFilter: string;
}) {
	const permission = useBusinessRequestPermission();
	const { favorites, handleAccessError, isError, isLoading } = useFavorites();
	const hasFavorites = favorites.size > 0;
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
	const allRows = table?.rows.filter((row) => favorites.has(row.symbol)) ?? [];
	const analysisFailed = hasFavorites && query.isError && !query.data;
	const rows = filterMarketScanRows(allRows, symbolFilter);

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap="md">
				<PageNavigation current="favorites" title="Favorites" />
				<SettingsForm {...settingsForm} />
				{isLoading || (query.isPending && hasFavorites) ? (
					<Center mih={180}>
						<Loader aria-label="Loading favorites" />
					</Center>
				) : null}
				{isError && !hasFavorites ? (
					<Paper p="xl" ta="center">
						<Text fw={600}>Unable to load favorites.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Try again when access and the backend are available.
						</Text>
					</Paper>
				) : null}
				{analysisFailed ? (
					<Paper p="xl" ta="center">
						<Text fw={600}>Unable to analyze favorites.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Try again when market data is available.
						</Text>
						<Button
							loading={query.isFetching}
							mt="md"
							onClick={() => void query.refetch()}
							variant="light"
						>
							Try again
						</Button>
					</Paper>
				) : null}
				{!isLoading && !isError && !hasFavorites ? (
					<Paper p="xl" ta="center">
						<Text fw={600}>No favorites yet.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Use the star in a market table to add one.
						</Text>
					</Paper>
				) : null}
				{table && allRows.length > 0 ? (
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
								criteria={scanCriteria}
								onSortChange={onSortChange}
								rows={rows}
								sort={sort}
								table={table}
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
