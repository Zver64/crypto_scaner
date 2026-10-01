import {
	Center,
	Container,
	Loader,
	Paper,
	Stack,
	Text,
	TextInput,
	useMatches,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useEffect, useState } from "react";
import { useAnalyzeMarket } from "@/api/generated/api";
import type { MarketAnalysisResponse } from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { SettingsForm } from "@/components/settings-form";
import {
	apiErrorMessage,
	unexpectedApiError,
} from "@/features/analysis/api-error";
import { hasExpectedMarketScanResult } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";
import { useVolatilitySettingsForm } from "@/features/analysis/volatility-settings-form/use-volatility-settings-form";
import { MarketScanResultsTable } from "@/features/market-scan/results-table";
import { filterMarketScanRows } from "@/features/market-scan/results-table/utils";
import type { MarketScanSort } from "@/features/market-scan/sort";
import {
	buildTopCoinsCriteria,
	buildTopCoinsScanCriteria,
	topCoinsRequestOptions,
} from "@/features/top-coins/top-coins";

interface TopCoinsScreenProps {
	initialSettings: VolatilitySettings;
	onSettingsCommit(settings: VolatilitySettings): void;
	onSortChange(sort: MarketScanSort): void;
	onSymbolFilterChange(symbolFilter: string): void;
	sort: MarketScanSort | undefined;
	symbolFilter: string;
}

export function TopCoinsScreen({
	initialSettings,
	onSettingsCommit,
	onSortChange,
	onSymbolFilterChange,
	sort,
	symbolFilter,
}: TopCoinsScreenProps) {
	const pageGap = useMatches({ base: "sm", sm: "md" });
	const permission = useBusinessRequestPermission();
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
	const query = useAnalyzeMarket<MarketAnalysisResponse>(
		{ criteria, ...topCoinsRequestOptions },
		{
			query: {
				enabled: permission.allowed,
				gcTime: Number.POSITIVE_INFINITY,
				retry: false,
				staleTime: Number.POSITIVE_INFINITY,
				select: (response) => {
					if (!hasExpectedMarketScanResult(response.data, criteria)) {
						throw unexpectedApiError();
					}
					return response.data;
				},
			},
		},
	);
	const allRows = query.data?.table.rows ?? [];
	const rows = filterMarketScanRows(allRows, symbolFilter);
	useEffect(() => {
		if (query.isError) {
			notifications.show({
				id: "top-market-cap-error",
				autoClose: 5000,
				color: "red",
				message: apiErrorMessage(query.error),
				title: "Top Market Cap failed",
			});
		}
	}, [query.error, query.isError]);
	useAnalysisWarningNotification(
		query.data?.warnings,
		"Top Market Cap warning",
	);

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap={pageGap}>
				<PageNavigation current="top-coins" title="Top Market Cap" />
				<SettingsForm {...settingsForm} />
				{query.isFetching && !query.data ? (
					<Center mih={180}>
						<Loader aria-label="Loading Top Market Cap" />
					</Center>
				) : null}
				{query.error && !query.data ? (
					<Paper p="xl" ta="center">
						<Text fw={600}>Unable to load the top coins.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Try again when market data is available.
						</Text>
					</Paper>
				) : null}
				{query.data ? (
					allRows.length > 0 ? (
						<>
							<TextInput
								aria-label="Filter Top Market Cap by symbol"
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
									table={query.data.table}
									window={query.data.price_history_window}
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
					) : (
						<Paper p="xl" ta="center">
							<Text fw={600}>No market cap data is available.</Text>
						</Paper>
					)
				) : null}
			</Stack>
		</Container>
	);
}
