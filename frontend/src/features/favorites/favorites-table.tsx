import { Button, Center, Loader, Paper, Text, TextInput } from "@mantine/core";
import type { FavoritesAnalysis } from "@/features/favorites/use-favorites-analysis";
import { MarketScanResultsTable } from "@/features/market-scan/results-table";
import { filterMarketScanRows } from "@/features/market-scan/results-table/utils";
import type { MarketScanSort } from "@/features/market-scan/sort";

interface FavoritesTableProps {
	analysis: FavoritesAnalysis;
	onRowClick?(symbol: string): void;
	onSortChange(sort: MarketScanSort): void;
	onSymbolFilterChange(symbolFilter: string): void;
	sort: MarketScanSort | undefined;
	symbolFilter: string;
}

// The favorites table with its symbol filter, loading, error and empty states.
export function FavoritesTable({
	analysis: {
		allRows,
		hasFavorites,
		isError,
		isLoading,
		query,
		scanCriteria,
		table,
	},
	onRowClick,
	onSortChange,
	onSymbolFilterChange,
	sort,
	symbolFilter,
}: FavoritesTableProps) {
	const analysisFailed = hasFavorites && query.isError && !query.data;
	const rows = filterMarketScanRows(allRows, symbolFilter);
	return (
		<>
			{isLoading || (query.isPending && hasFavorites) ? (
				<Center mih={180}>
					<Loader aria-label="Loading favorites" />
				</Center>
			) : null}
			{isError && !hasFavorites ? (
				<Paper p="xl" ta="center">
					<Text fw={600}>Unable to load favorites.</Text>
					<Text c="dimmed" mt={4} size="sm">
						Try again when access and the backend are available.
					</Text>
				</Paper>
			) : null}
			{analysisFailed ? (
				<Paper p="xl" ta="center">
					<Text fw={600}>Unable to analyze favorites.</Text>
					<Text c="dimmed" mt={4} size="sm">
						Try again when market data is available.
					</Text>
					<Button
						loading={query.isFetching}
						mt="md"
						onClick={() => void query.refetch()}
						variant="light"
					>
						Try again
					</Button>
				</Paper>
			) : null}
			{!isLoading && !isError && !hasFavorites ? (
				<Paper p="xl" ta="center">
					<Text fw={600}>No favorites yet.</Text>
					<Text c="dimmed" mt={4} size="sm">
						Use the star in a market table to add one.
					</Text>
				</Paper>
			) : null}
			{table && allRows.length > 0 ? (
				<>
					<TextInput
						aria-label="Filter Favorites by symbol"
						label="Symbol filter"
						labelProps={{ mb: "xs" }}
						onChange={(event) =>
							onSymbolFilterChange(event.currentTarget.value)
						}
						placeholder="e.g. BTC"
						size="md"
						value={symbolFilter}
					/>
					{rows.length > 0 ? (
						<MarketScanResultsTable
							criteria={scanCriteria}
							onRowClick={onRowClick}
							onSortChange={onSortChange}
							rows={rows}
							sort={sort}
							table={table}
							window={query.data?.price_history_window}
						/>
					) : (
						<Paper p="xl" ta="center">
							<Text fw={600}>No instruments match this symbol filter.</Text>
							<Text c="dimmed" mt={4} size="sm">
								Clear or change the filter to see all instruments.
							</Text>
						</Paper>
					)}
				</>
			) : null}
		</>
	);
}
