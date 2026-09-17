import { Center, Container, Loader, Stack, useMatches } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { telegramRequestOptions } from "@/app/telegram";
import { type ApiError, apiErrorMessage } from "@/features/analysis/api-error";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { MarketScanForm } from "@/features/market-scan/form";
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
	sort: MarketScanSort;
	symbolFilter: string;
}

function showMarketScanError(error: ApiError) {
	notifications.show({
		id: "market-scan-error",
		autoClose: 5000,
		color: "red",
		message: apiErrorMessage(error),
		title: "Market Scan failed",
	});
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
	const permission = useBusinessRequestPermission();
	const queryClient = useQueryClient();
	const [committedCriteria, setCommittedCriteria] = useState(initialCriteria);
	const requestOptions = telegramRequestOptions();
	const query = useQuery({
		...marketScanObserverOptions(
			committedCriteria ?? defaultMarketScanCriteria,
			requestOptions,
		),
		enabled: permission.allowed && committedCriteria !== undefined,
	});
	const mutation = useMutation(
		marketScanMutationOptions(queryClient, requestOptions),
	);

	useEffect(() => {
		if (committedCriteria && query.isLoadingError) {
			showMarketScanError(query.error);
		}
	}, [committedCriteria, query.error, query.isLoadingError]);

	const displayedData = committedCriteria ? query.data : undefined;
	const isScanPending = query.isLoading || mutation.isPending;
	useAnalysisWarningNotification(
		displayedData?.warnings,
		"Market Scan warning",
	);

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap={pageGap}>
				<PageNavigation current="market-scan" title="Market Scan" />
				<MarketScanForm
					initialCriteria={initialCriteria ?? defaultMarketScanCriteria}
					disabled={!permission.allowed || isScanPending}
					isSubmitting={isScanPending}
					onCommit={(criteria) => {
						mutation.mutate(criteria, {
							onError: showMarketScanError,
							onSuccess: () => {
								setCommittedCriteria(criteria);
								onCriteriaCommit(criteria);
							},
						});
						return Promise.resolve();
					}}
				/>
				{isScanPending && !displayedData ? (
					<Center mih={180}>
						<Loader aria-label="Loading Market Scan" />
					</Center>
				) : null}
				{committedCriteria && displayedData ? (
					<MarketScanResults
						criteria={committedCriteria}
						isRefreshing={isScanPending}
						onSortChange={onSortChange}
						onSymbolFilterChange={onSymbolFilterChange}
						result={displayedData}
						sort={sort}
						symbolFilter={symbolFilter}
					/>
				) : null}
			</Stack>
		</Container>
	);
}
