import {
	Group,
	Paper,
	Stack,
	Text,
	TextInput,
	Title,
	useMatches,
} from "@mantine/core";
import { useState } from "react";
import type { MarketScanResult } from "@/api/client";
import { DataTable } from "@/components/data-table";
import { RefreshingOverlay } from "@/components/refreshing-overlay";
import type { MarketScanCriteria } from "@/features/market-scan/pipeline";
import { MarketScanResultsTable } from "@/features/market-scan/results-table";
import { unresolvedInstrumentColumns } from "@/features/market-scan/results-table/columns";
import {
	filterMarketScanRows,
	toMarketScanRows,
} from "@/features/market-scan/results-table/utils";
import type { MarketScanSort } from "@/features/market-scan/sort";

interface MarketScanResultsProps {
	criteria: MarketScanCriteria;
	isRefreshing: boolean;
	onSelectInstrument(
		symbol: string,
		criteria: MarketScanCriteria,
	): Promise<void>;
	onSortChange(sort: MarketScanSort): Promise<void>;
	result: MarketScanResult;
	sort: MarketScanSort;
}

export function MarketScanResults({
	criteria,
	isRefreshing,
	onSelectInstrument,
	onSortChange,
	result,
	sort,
}: MarketScanResultsProps) {
	const contentSpacing = useMatches({ base: "xs", sm: "sm" });
	const textSize = useMatches({ base: "xs", sm: "sm" });
	const [symbolFilter, setSymbolFilter] = useState("");
	const rows = filterMarketScanRows(
		toMarketScanRows(result.items),
		symbolFilter,
	);

	return (
		<RefreshingOverlay label="Refreshing Market Scan" visible={isRefreshing}>
			<Stack gap={contentSpacing}>
				<Paper p={contentSpacing} withBorder>
					<Group gap="lg">
						<Text size={textSize}>
							Matched <Text component="strong">{result.matched_count}</Text>
						</Text>
						<Text size={textSize}>
							Analyzed <Text component="strong">{result.analyzed_count}</Text>
						</Text>
						<Text size={textSize}>
							Insufficient data{" "}
							<Text component="strong">{result.insufficient_data_count}</Text>
						</Text>
					</Group>
				</Paper>
				<TextInput
					aria-label="Filter current Scan Result by symbol"
					description="Filters only the current Scan Result"
					label="Symbol filter"
					size={textSize}
					onChange={(event) => setSymbolFilter(event.currentTarget.value)}
					placeholder="e.g. BTC"
					value={symbolFilter}
				/>
				{result.items.length === 0 ? (
					<Paper p="xl" ta="center" withBorder>
						<Text fw={600}>No instruments matched these criteria.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Adjust the criteria and run another Market Scan.
						</Text>
					</Paper>
				) : rows.length === 0 ? (
					<Paper p="xl" ta="center" withBorder>
						<Text fw={600}>No instruments match this symbol filter.</Text>
						<Text c="dimmed" mt={4} size="sm">
							Clear or change the filter to see this Scan Result.
						</Text>
					</Paper>
				) : (
					<MarketScanResultsTable
						rows={rows}
						window={result.price_history_window}
						sort={sort}
						onSortChange={onSortChange}
						onSelectInstrument={(symbol) =>
							void onSelectInstrument(symbol, criteria)
						}
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
							minWidth={420}
							rows={result.unresolved}
						/>
					</Stack>
				) : null}
			</Stack>
		</RefreshingOverlay>
	);
}
