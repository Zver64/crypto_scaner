import { Center, Loader, useMatches } from "@mantine/core";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { showQueryErrorNotification } from "@/app/query-client";
import { EmptyState } from "@/components/empty-state";
import { PageStack } from "@/components/page-stack";
import { SidebarLayout } from "@/components/sidebar-layout";
import { sidebarColumns } from "@/components/sidebar-layout/constants";
import { useWideLayout } from "@/components/sidebar-layout/use-wide-layout";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { MarketScanForm } from "@/features/market-scan/form";
import { marketScanErrorNotification } from "@/features/market-scan/market-scan-screen/constants";
import {
	marketScanMutationOptions,
	marketScanObserverOptions,
} from "@/features/market-scan/market-scan-screen/utils";
import {
	defaultMarketScanCriteria,
	type MarketScanCriteria,
} from "@/features/market-scan/pipeline";
import { MarketScanResults } from "@/features/market-scan/results-view";
import type { MarketScanSort } from "@/features/market-scan/sort";

interface MarketScanScreenProps {
	initialCriteria: MarketScanCriteria | undefined;
	onCriteriaCommit(criteria: MarketScanCriteria): void;
	onSortChange(sort: MarketScanSort): void;
	onSymbolFilterChange(symbolFilter: string): void;
	sort: MarketScanSort | undefined;
	symbolFilter: string;
}

export function MarketScanScreen({
	initialCriteria,
	onCriteriaCommit,
	onSortChange,
	onSymbolFilterChange,
	sort,
	symbolFilter,
}: MarketScanScreenProps) {
	const pageGap = useMatches({ base: "sm", sm: "md" });
	const wide = useWideLayout();
	const permission = useBusinessRequestPermission();
	const queryClient = useQueryClient();
	const [committedCriteria, setCommittedCriteria] = useState(initialCriteria);
	const query = useQuery({
		...marketScanObserverOptions(
			committedCriteria ?? defaultMarketScanCriteria,
		),
		enabled: permission.allowed && committedCriteria !== undefined,
	});
	const mutation = useMutation(marketScanMutationOptions(queryClient));

	const displayedData = committedCriteria ? query.data : undefined;
	const isScanPending = query.isLoading || mutation.isPending;
	useAnalysisWarningNotification(
		displayedData?.warnings,
		"Market Scan warning",
	);

	return (
		<PageStack gap={pageGap}>
			<PageNavigation current="market-scan" title="Market Scan" />
			<SidebarLayout
				gap={pageGap}
				sidebar={
					<MarketScanForm
						columns={sidebarColumns}
						initialCriteria={initialCriteria ?? defaultMarketScanCriteria}
						disabled={!permission.allowed || isScanPending}
						isSubmitting={isScanPending}
						onCommit={(criteria) => {
							mutation.mutate(criteria, {
								onError: (error) =>
									showQueryErrorNotification(
										marketScanErrorNotification,
										error,
									),
								onSuccess: () => {
									setCommittedCriteria(criteria);
									onCriteriaCommit(criteria);
								},
							});
							return Promise.resolve();
						}}
					/>
				}
				sidebarPosition="start"
			>
				{isScanPending && !displayedData ? (
					<Center mih={180}>
						<Loader aria-label="Loading Market Scan" />
					</Center>
				) : null}
				{/* Phones show the form alone; beside the sidebar the main area
				explains why it is empty. */}
				{wide && !isScanPending && !displayedData ? (
					<EmptyState
						description={
							!permission.allowed
								? "Try again when the backend is available."
								: query.isError
									? "Adjust the criteria or run the Market Scan again."
									: "Set the criteria and run a Market Scan to see matching instruments."
						}
						failed={permission.allowed && query.isError}
						fillHeight
						title={
							!permission.allowed
								? "Market Scan is unavailable."
								: query.isError
									? "Market Scan failed."
									: "No Scan Result yet."
						}
					/>
				) : null}
				{committedCriteria && displayedData ? (
					<MarketScanResults
						criteria={committedCriteria}
						fillHeight={wide}
						isRefreshing={isScanPending}
						onSortChange={onSortChange}
						onSymbolFilterChange={onSymbolFilterChange}
						result={displayedData}
						sort={sort}
						symbolFilter={symbolFilter}
					/>
				) : null}
			</SidebarLayout>
		</PageStack>
	);
}
