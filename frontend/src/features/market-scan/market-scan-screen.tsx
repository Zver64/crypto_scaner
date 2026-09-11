import { Center, Container, Loader, Stack, useMatches } from "@mantine/core";
import { useCallback, useEffect, useRef, useState } from "react";
import type { MarketScanResult } from "@/api/client";
import { useMarketScanMutation } from "@/api/market-scan";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { useAnalysisErrorNotification } from "@/features/analysis/use-analysis-error-notification";
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
	const { error, isPending, mutateAsync } = useMarketScanMutation();
	const initialScanPending = useRef(initialCriteria);
	const [displayedScan, setDisplayedScan] = useState<
		| {
				criteria: MarketScanCriteria;
				result: MarketScanResult;
		  }
		| undefined
	>();

	const runScan = useCallback(
		async (criteria: MarketScanCriteria) => {
			try {
				const result = await mutateAsync(criterionSelections(criteria));
				setDisplayedScan({ criteria, result });
			} catch {
				// The mutation state is rendered through the shared error notification.
			}
		},
		[mutateAsync],
	);

	useEffect(() => {
		if (!permission.allowed || !initialScanPending.current) return;

		const criteria = initialScanPending.current;
		initialScanPending.current = undefined;
		void runScan(criteria);
	}, [permission.allowed, runScan]);

	useAnalysisErrorNotification(error, "Market Scan failed");
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
					disabled={!permission.allowed || isPending}
					isSubmitting={isPending}
					onCommit={async (criteria) => {
						onCriteriaCommit(criteria);
						await runScan(criteria);
					}}
				/>
				{isPending && !displayedScan ? (
					<Center mih={180}>
						<Loader aria-label="Loading Market Scan" />
					</Center>
				) : null}
				{displayedScan ? (
					<MarketScanResults
						criteria={displayedScan.criteria}
						isRefreshing={isPending}
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
