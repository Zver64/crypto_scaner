import {
	Center,
	Container,
	Loader,
	Paper,
	Stack,
	Text,
	useMatches,
} from "@mantine/core";
import { useState } from "react";
import { useMarketScanQuery } from "@/api/market-scan";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { PageNavigation } from "@/app/page-navigation";
import { useAnalysisErrorNotification } from "@/features/analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { MarketScanResultsTable } from "@/features/market-scan/results-table";
import { defaultMarketScanSort } from "@/features/market-scan/sort";
import {
	topCoinsCriteria,
	topCoinsRequestOptions,
	toTopCoinRows,
} from "@/features/top-coins/top-coins";

export function TopCoinsScreen() {
	const pageGap = useMatches({ base: "sm", sm: "md" });
	const permission = useBusinessRequestPermission();
	const query = useMarketScanQuery(
		topCoinsCriteria,
		permission.allowed,
		topCoinsRequestOptions,
	);
	const [sort, setSort] = useState(defaultMarketScanSort);
	const rows = toTopCoinRows(query.data?.items ?? []);
	useAnalysisErrorNotification(query.error, "Top Market Cap failed");
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
