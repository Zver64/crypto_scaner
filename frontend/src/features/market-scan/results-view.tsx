import {
	Group,
	Paper,
	Stack,
	Text,
	TextInput,
	Title,
	Tooltip,
	useMatches,
} from "@mantine/core";
import type {
	InsufficientDataInstrument,
	MarketAnalysisResponse,
} from "@/api/generated/models";
import { DataTable } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import type { MarketScanCriteria } from "@/features/market-scan/pipeline";
import { MarketScanResultsTable } from "@/features/market-scan/results-table";
import { unresolvedInstrumentColumns } from "@/features/market-scan/results-table/columns";
import { filterMarketScanRows } from "@/features/market-scan/results-table/utils";
import type { MarketScanSort } from "@/features/market-scan/sort";

const unitSuffixes: Record<InsufficientDataInstrument["unit"], string> = {
	days: "d",
	hours: "h",
};

function formatInsufficientData(
	instruments: readonly InsufficientDataInstrument[],
): string {
	return instruments
		.map(
			({ available, required, symbol, unit }) =>
				`${symbol} ${available}/${required}${unitSuffixes[unit]}`,
		)
		.join(", ");
}

interface MarketScanResultsProps {
	criteria: MarketScanCriteria;
	// Lets the results and their empty states fill the free height of a flex
	// column parent.
	fillHeight?: boolean;
	isRefreshing: boolean;
	onSortChange(sort: MarketScanSort): void;
	onSymbolFilterChange(symbolFilter: string): void;
	result: MarketAnalysisResponse;
	sort: MarketScanSort | undefined;
	symbolFilter: string;
}

export function MarketScanResults({
	criteria,
	fillHeight = false,
	isRefreshing,
	onSortChange,
	onSymbolFilterChange,
	result,
	sort,
	symbolFilter,
}: MarketScanResultsProps) {
	const contentSpacing = useMatches({ base: "xs", sm: "sm" });
	const textSize = useMatches({ base: "xs", sm: "sm" });
	const rows = filterMarketScanRows(result.table.rows, symbolFilter);

	return (
		<RefreshingOverlay
			fillHeight={fillHeight}
			label="Refreshing Market Scan"
			visible={isRefreshing}
		>
			<Stack flex={fillHeight ? 1 : undefined} gap={contentSpacing}>
				<Paper p={contentSpacing}>
					<Group gap="lg">
						<Text size={textSize}>
							Matched <Text component="strong">{result.matched_count}</Text>
						</Text>
						<Text size={textSize}>
							Analyzed <Text component="strong">{result.analyzed_count}</Text>
						</Text>
						<Tooltip
							disabled={result.insufficient_data.length === 0}
							events={{ focus: true, hover: true, touch: true }}
							label={formatInsufficientData(result.insufficient_data)}
							maw={320}
							multiline
						>
							<Text size={textSize}>
								Insufficient data{" "}
								<Text component="strong">{result.insufficient_data_count}</Text>
							</Text>
						</Tooltip>
					</Group>
				</Paper>
				<TextInput
					aria-label="Filter current Scan Result by symbol"
					onChange={(event) => onSymbolFilterChange(event.currentTarget.value)}
					placeholder="Filter by symbol, e.g. BTC"
					value={symbolFilter}
				/>
				{result.table.rows.length === 0 ? (
					<EmptyState
						description="Adjust the criteria and run another Market Scan."
						fillHeight={fillHeight}
						title="No instruments matched these criteria."
					/>
				) : rows.length === 0 ? (
					<EmptyState
						description="Clear or change the filter to see this Scan Result."
						fillHeight={fillHeight}
						title="No instruments match this symbol filter."
					/>
				) : (
					<MarketScanResultsTable
						criteria={criteria}
						onSortChange={onSortChange}
						rows={rows}
						sort={sort}
						table={result.table}
						window={result.price_history_window}
					/>
				)}
				{result.unresolved.length > 0 ? (
					<Stack gap="xs">
						<Title order={2} size={textSize === "xs" ? "h5" : "h4"}>
							Instruments with unavailable Market Cap
						</Title>
						<DataTable
							columns={unresolvedInstrumentColumns}
							getRowKey={(item) => `${item.symbol}-${item.code}`}
							rows={result.unresolved}
						/>
					</Stack>
				) : null}
			</Stack>
		</RefreshingOverlay>
	);
}
