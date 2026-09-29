import type {
	MarketScanSort,
	MarketScanSortDirection,
} from "@/features/market-scan/sort";
import {
	parseOptionalScanCriteriaSearch,
	type ScanCriteriaSearch,
} from "@/routes/-scan-criteria-search";

export interface MarketScanSortSearch {
	sort_column?: string;
	sort_direction?: MarketScanSortDirection;
}

export interface SymbolFilterSearch {
	symbol_filter?: string;
}

export interface MarketScanSearch
	extends ScanCriteriaSearch,
		MarketScanSortSearch,
		SymbolFilterSearch {}

// Sortable columns come from the backend table, so the column is checked
// against the table once it is loaded.
export function parseMarketScanSortSearch(
	search: Record<string, unknown>,
): MarketScanSortSearch {
	const sortColumn = search.sort_column;
	const sortDirection = search.sort_direction;
	if (
		typeof sortColumn === "string" &&
		sortColumn.length > 0 &&
		(sortDirection === "asc" || sortDirection === "desc")
	) {
		return {
			sort_column: sortColumn,
			sort_direction: sortDirection,
		};
	}

	return {};
}

export function parseMarketScanSearch(
	search: Record<string, unknown>,
): MarketScanSearch {
	return {
		...parseOptionalScanCriteriaSearch(search),
		...parseMarketScanSortSearch(search),
		...parseSymbolFilterSearch(search),
	};
}

export function parseSymbolFilterSearch(
	search: Record<string, unknown>,
): SymbolFilterSearch {
	const symbolFilter = search.symbol_filter;
	return typeof symbolFilter === "string" && symbolFilter.length > 0
		? { symbol_filter: symbolFilter }
		: {};
}

export function marketScanSortFromSearch(
	search: MarketScanSortSearch,
): MarketScanSort | undefined {
	return search.sort_column && search.sort_direction
		? { column: search.sort_column, direction: search.sort_direction }
		: undefined;
}
