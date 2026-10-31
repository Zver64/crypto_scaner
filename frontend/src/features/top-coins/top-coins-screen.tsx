import { Center, Loader, Stack, TextInput, useMatches } from "@mantine/core";
import { useState } from "react";

import {
	analyzeMarket,
	type analyzeMarketResponseSuccess,
	useAnalyzeMarket,
} from "@/api/generated/api";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { EmptyState } from "@/components/empty-state";
import { PageStack } from "@/components/page-stack";
import { SidebarLayout } from "@/components/sidebar-layout";
import { sidebarColumns } from "@/components/sidebar-layout/constants";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { unexpectedApiError } from "@/features/analysis/api-error";
import { hasExpectedMarketScanResult } from "@/features/analysis/semantics";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { VolatilitySettingsForm } from "@/features/analysis/volatility-settings-form";
import type { VolatilitySettingsStateProps } from "@/features/analysis/volatility-settings-form/types";
import { unfilteredScanCriteria } from "@/features/market-scan/pipeline";
import { MarketScanResultsTable } from "@/features/market-scan/results-table";
import type { ResultsTableStateProps } from "@/features/market-scan/results-table/types";
import { filterMarketScanRows } from "@/features/market-scan/results-table/utils";
import {
	buildTopCoinsCriteria,
	topCoinsPages,
} from "@/features/top-coins/top-coins";

const selectAnalysis = (response: analyzeMarketResponseSuccess) =>
	response.data;

interface TopCoinsScreenProps
	extends VolatilitySettingsStateProps, ResultsTableStateProps {
	page: keyof typeof topCoinsPages;
}

export function TopCoinsScreen({
	initialSettings,
	onSettingsCommit,
	onSortChange,
	onSymbolFilterChange,
	page,
	sort,
	symbolFilter,
}: TopCoinsScreenProps) {
	const {
		highlightCoinM,
		request: requestOptions,
		title,
	} = topCoinsPages[page];
	const pageGap = useMatches({ base: "sm", sm: "md" });
	const wide = useWideLayout();
	const permission = useBusinessRequestPermission();
	const [settings, setSettings] = useState(initialSettings);
	const scanCriteria = unfilteredScanCriteria(settings);
	const criteria = buildTopCoinsCriteria(settings);
	const request = { criteria, ...requestOptions };
	const query = useAnalyzeMarket(request, {
		query: {
			enabled: permission.allowed,
			gcTime: Number.POSITIVE_INFINITY,
			meta: {
				errorNotification: {
					id: `${page}-error`,
					title: `${title} failed`,
				},
			},
			// Validated when fetched, so an unexpected response fails the query
			// and reaches the query cache error handler.
			queryFn: async ({ signal }) => {
				const response = await analyzeMarket(request, { signal });
				if (!hasExpectedMarketScanResult(response.data, criteria)) {
					throw unexpectedApiError();
				}
				return response;
			},
			retry: false,
			staleTime: Number.POSITIVE_INFINITY,
			select: selectAnalysis,
		},
	});
	const allRows = query.data?.table.rows ?? [];
	const rows = filterMarketScanRows(allRows, symbolFilter);
	useAnalysisWarningNotification(query.data?.warnings, `${title} warning`);

	return (
		<PageStack gap={pageGap}>
			<PageNavigation current={page} title={title} />
			<SidebarLayout
				gap={pageGap}
				sidebar={
					<VolatilitySettingsForm
						columns={sidebarColumns}
						disabled={!permission.allowed}
						initialSettings={initialSettings}
						loading={query.isFetching}
						onCommit={(nextSettings) => {
							setSettings(nextSettings);
							onSettingsCommit(nextSettings);
						}}
					/>
				}
				sidebarPosition="start"
			>
				<Stack flex={wide ? 1 : undefined} gap={pageGap}>
					{query.isFetching && !query.data ? (
						<Center mih={180}>
							<Loader aria-label={`Loading ${title}`} />
						</Center>
					) : null}
					{query.error && !query.data ? (
						<EmptyState
							description="Try again when market data is available."
							fillHeight={wide}
							title="Unable to load the top coins."
						/>
					) : null}
					{query.data ? (
						allRows.length > 0 ? (
							<>
								<TextInput
									aria-label={`Filter ${title} by symbol`}
									onChange={(event) =>
										onSymbolFilterChange(event.currentTarget.value)
									}
									placeholder="Filter by symbol, e.g. BTC"
									value={symbolFilter}
								/>
								{rows.length > 0 ? (
									<MarketScanResultsTable
										criteria={scanCriteria}
										highlightCoinM={highlightCoinM}
										onSortChange={onSortChange}
										rows={rows}
										sort={sort}
										table={query.data.table}
									/>
								) : (
									<EmptyState
										description="Clear or change the filter to see all instruments."
										fillHeight={wide}
										title="No instruments match this symbol filter."
									/>
								)}
							</>
						) : (
							<EmptyState
								fillHeight={wide}
								title="No market cap data is available."
							/>
						)
					) : null}
				</Stack>
			</SidebarLayout>
		</PageStack>
	);
}
