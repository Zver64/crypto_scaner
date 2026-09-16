import {
	Center,
	Container,
	Loader,
	Paper,
	Stack,
	Text,
	useMatches,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useEffect, useState } from "react";
import { useAnalyzeMarket } from "@/api/generated/api";
import type { MarketAnalysisResponse } from "@/api/generated/models";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { telegramRequestOptions } from "@/app/telegram";
import {
	apiErrorMessage,
	unexpectedApiError,
} from "@/features/analysis/api-error";
import { hasExpectedMarketScanResult } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { MarketScanResultsTable } from "@/features/market-scan/results-table";
import { defaultMarketScanSort } from "@/features/market-scan/sort";
import {
	topCoinsCriteria,
	topCoinsRequestOptions,
	topCoinsScanCriteria,
	toTopCoinRows,
} from "@/features/top-coins/top-coins";

export function TopCoinsScreen() {
	const pageGap = useMatches({ base: "sm", sm: "md" });
	const permission = useBusinessRequestPermission();
	const query = useAnalyzeMarket<MarketAnalysisResponse>(
		{ criteria: [...topCoinsCriteria], ...topCoinsRequestOptions },
		{
			fetch: telegramRequestOptions(),
			query: {
				enabled: permission.allowed,
				gcTime: Number.POSITIVE_INFINITY,
				retry: false,
				staleTime: Number.POSITIVE_INFINITY,
				select: (response) => {
					if (!hasExpectedMarketScanResult(response.data, topCoinsCriteria)) {
						throw unexpectedApiError();
					}
					return response.data;
				},
			},
		},
	);
	const [sort, setSort] = useState(defaultMarketScanSort);
	const rows = toTopCoinRows(query.data?.items ?? []);
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
					rows.length > 0 ? (
						<MarketScanResultsTable
							criteria={topCoinsScanCriteria}
							onSortChange={setSort}
							rows={rows}
							sort={sort}
							window={query.data.price_history_window}
						/>
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
