import { Button, Center, Loader, TextInput } from "@mantine/core";
import { EmptyState } from "@/components/empty-state";
import type { FavoritesAnalysis } from "@/features/favorites/use-favorites-analysis";
import { MarketScanResultsTable } from "@/features/market-scan/results-table";
import { filterMarketScanRows } from "@/features/market-scan/results-table/utils";
import type { MarketScanSort } from "@/features/market-scan/sort";

interface FavoritesTableProps {
	analysis: FavoritesAnalysis;
	// Lets empty states fill the free height of a flex column parent.
	fillHeight?: boolean;
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
	fillHeight = false,
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
				<EmptyState
					description="Try again when access and the backend are available."
					fillHeight={fillHeight}
					title="Unable to load favorites."
				/>
			) : null}
			{analysisFailed ? (
				<EmptyState
					description="Try again when market data is available."
					fillHeight={fillHeight}
					title="Unable to analyze favorites."
				>
					<Button
						loading={query.isFetching}
						mt="md"
						onClick={() => void query.refetch()}
						variant="light"
					>
						Try again
					</Button>
				</EmptyState>
			) : null}
			{!isLoading && !isError && !hasFavorites ? (
				<EmptyState
					description="Use the star in a market table to add one."
					fillHeight={fillHeight}
					title="No favorites yet."
				/>
			) : null}
			{table && allRows.length > 0 ? (
				<>
					<TextInput
						aria-label="Filter Favorites by symbol"
						onChange={(event) =>
							onSymbolFilterChange(event.currentTarget.value)
						}
						placeholder="Filter by symbol, e.g. BTC"
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
						<EmptyState
							description="Clear or change the filter to see all instruments."
							fillHeight={fillHeight}
							title="No instruments match this symbol filter."
						/>
					)}
				</>
			) : null}
		</>
	);
}
