import { Center, Container, Loader, Stack, useMatches } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useCallback, useEffect, useRef, useState } from "react";
import {
	getAnalyzeMarketQueryKey,
	useAnalyzeMarket,
} from "@/api/generated/api";
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
import { MarketScanForm } from "@/features/market-scan/form";
import {
	criterionSelections,
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
	const initialScanPending = useRef(initialCriteria);
	const [requestedCriteria, setRequestedCriteria] = useState<
		MarketScanCriteria | undefined
	>();
	const [submissionId, setSubmissionId] = useState(0);
	const [displayedScan, setDisplayedScan] = useState<
		| {
				criteria: MarketScanCriteria;
				result: MarketAnalysisResponse;
		  }
		| undefined
	>();
	const selections = requestedCriteria
		? criterionSelections(requestedCriteria)
		: [];
	const request = { criteria: selections };
	const query = useAnalyzeMarket<MarketAnalysisResponse>(request, {
		fetch: telegramRequestOptions(),
		query: {
			enabled: permission.allowed && requestedCriteria !== undefined,
			queryKey: [...getAnalyzeMarketQueryKey(request), submissionId],
			retry: false,
			select: (response) => {
				if (!hasExpectedMarketScanResult(response.data, selections)) {
					throw unexpectedApiError();
				}
				return response.data;
			},
		},
	});

	const runScan = useCallback((criteria: MarketScanCriteria) => {
		setRequestedCriteria(criteria);
		setSubmissionId((current) => current + 1);
	}, []);

	useEffect(() => {
		if (!permission.allowed || !initialScanPending.current) return;
		const criteria = initialScanPending.current;
		initialScanPending.current = undefined;
		runScan(criteria);
	}, [permission.allowed, runScan]);

	useEffect(() => {
		if (requestedCriteria && query.data) {
			setDisplayedScan({ criteria: requestedCriteria, result: query.data });
		}
	}, [query.data, requestedCriteria]);

	useEffect(() => {
		if (query.isError) {
			notifications.show({
				id: "market-scan-error",
				autoClose: 5000,
				color: "red",
				message: apiErrorMessage(query.error),
				title: "Market Scan failed",
			});
		}
	}, [query.error, query.isError]);

	useAnalysisWarningNotification(
		displayedScan?.result.warnings,
		"Market Scan warning",
	);

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap={pageGap}>
				<PageNavigation current="market-scan" title="Market Scan" />
				<MarketScanForm
					initialCriteria={initialCriteria ?? defaultMarketScanCriteria}
					disabled={!permission.allowed || query.isFetching}
					isSubmitting={query.isFetching}
					onCommit={async (criteria) => {
						onCriteriaCommit(criteria);
						runScan(criteria);
					}}
				/>
				{query.isFetching && !displayedScan ? (
					<Center mih={180}>
						<Loader aria-label="Loading Market Scan" />
					</Center>
				) : null}
				{displayedScan ? (
					<MarketScanResults
						criteria={displayedScan.criteria}
						isRefreshing={query.isFetching}
						onSortChange={onSortChange}
						onSymbolFilterChange={onSymbolFilterChange}
						result={displayedScan.result}
						sort={sort}
						symbolFilter={symbolFilter}
					/>
				) : null}
			</Stack>
		</Container>
	);
}
