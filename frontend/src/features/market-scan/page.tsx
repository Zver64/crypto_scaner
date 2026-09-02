import { Center, Container, Loader, Stack, Text, Title } from "@mantine/core";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { useAnalysisErrorNotification } from "@/features/analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { MarketScanForm } from "./form";
import type { MarketScanCriteria } from "./pipeline";
import { useMarketScanQuery } from "./query";
import { MarketScanResults } from "./results-view";
import type { MarketScanSort } from "./sort";

export { marketScanQueryKey } from "./query";

interface MarketScanPageProps {
	committedCriteria: MarketScanCriteria | undefined;
	onCommit(criteria: MarketScanCriteria): Promise<void>;
	onSortChange(sort: MarketScanSort): Promise<void>;
	onSelectInstrument(
		symbol: string,
		criteria: MarketScanCriteria,
	): Promise<void>;
	sort: MarketScanSort;
}

export function MarketScanPage({
	committedCriteria,
	onCommit,
	onSortChange,
	onSelectInstrument,
	sort,
}: MarketScanPageProps) {
	const permission = useBusinessRequestPermission();
	const query = useMarketScanQuery(committedCriteria, permission.allowed);
	useAnalysisErrorNotification(query.error, "Market Scan failed");
	useAnalysisWarningNotification(query.data?.warnings, "Market Scan warning");

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap="md">
				<PageHeading />
				<MarketScanForm
					committedCriteria={committedCriteria}
					disabled={!permission.allowed || query.isFetching}
					isSubmitting={query.isFetching}
					onCommit={onCommit}
					onRefresh={query.refetch}
				/>
				{committedCriteria && query.isFetching && !query.data ? (
					<Center mih={180}>
						<Loader aria-label="Loading Market Scan" />
					</Center>
				) : null}
				{query.data && committedCriteria ? (
					<MarketScanResults
						criteria={committedCriteria}
						isRefreshing={query.isFetching}
						onSelectInstrument={onSelectInstrument}
						onSortChange={onSortChange}
						result={query.data}
						sort={sort}
					/>
				) : null}
			</Stack>
		</Container>
	);
}

function PageHeading() {
	return (
		<Stack gap={2}>
			<Title order={1} size="h2">
				Market Scan
			</Title>
			<Text c="dimmed" size="sm">
				Evaluate daily volatility, then hourly volatility, then each enabled
				criterion.
			</Text>
		</Stack>
	);
}
