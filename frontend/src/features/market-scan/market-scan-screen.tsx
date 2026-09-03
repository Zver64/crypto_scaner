import {
	Center,
	Container,
	Loader,
	Stack,
	Text,
	Title,
	useMatches,
} from "@mantine/core";
import { useMarketScanQuery } from "@/api/market-scan";
import { useBusinessRequestPermission } from "@/app/business-request-context";
import { useAnalysisErrorNotification } from "@/features/analysis/use-analysis-error-notification";
import { useAnalysisWarningNotification } from "@/features/analysis/use-analysis-warning-notification";
import { MarketScanForm } from "@/features/market-scan/form";
import {
	criterionSelections,
	type MarketScanCriteria,
} from "@/features/market-scan/pipeline";
import { MarketScanResults } from "@/features/market-scan/results-view";
import type { MarketScanSort } from "@/features/market-scan/sort";

interface MarketScanScreenProps {
	committedCriteria: MarketScanCriteria | undefined;
	onCommit(criteria: MarketScanCriteria): Promise<void>;
	onSortChange(sort: MarketScanSort): Promise<void>;
	onSelectInstrument(
		symbol: string,
		criteria: MarketScanCriteria,
	): Promise<void>;
	sort: MarketScanSort;
}

export function MarketScanScreen({
	committedCriteria,
	onCommit,
	onSortChange,
	onSelectInstrument,
	sort,
}: MarketScanScreenProps) {
	const pageGap = useMatches({ base: "sm", sm: "md" });
	const permission = useBusinessRequestPermission();
	const query = useMarketScanQuery(
		committedCriteria ? criterionSelections(committedCriteria) : undefined,
		permission.allowed,
	);
	useAnalysisErrorNotification(query.error, "Market Scan failed");
	useAnalysisWarningNotification(query.data?.warnings, "Market Scan warning");

	return (
		<Container maw={880} px={0} size="md">
			<Stack gap={pageGap}>
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
	const headingSize = useMatches({ base: "h3", sm: "h2" });
	const descriptionSize = useMatches({ base: "xs", sm: "sm" });

	return (
		<Stack gap={2}>
			<Title order={1} size={headingSize}>
				Market Scan
			</Title>
			<Text c="dimmed" size={descriptionSize}>
				Evaluate daily volatility, then hourly volatility, then each enabled
				criterion.
			</Text>
		</Stack>
	);
}
