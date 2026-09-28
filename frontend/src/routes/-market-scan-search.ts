import type { MarketScanSortColumn } from "@/features/market-scan/results-table/columns";
import { marketScanColumnKeys } from "@/features/market-scan/results-table/keys";
import {
	defaultMarketScanSort,
	type MarketScanSort,
	type MarketScanSortDirection,
} from "@/features/market-scan/sort";
import {
	parseOptionalScanCriteriaSearch,
	type ScanCriteriaSearch,
} from "@/routes/-scan-criteria-search";

export interface MarketScanSortSearch {
	sort_column?: MarketScanSortColumn;
	sort_direction?: MarketScanSortDirection;
}

export interface SymbolFilterSearch {
	symbol_filter?: string;
}

export interface MarketScanSearch
	extends ScanCriteriaSearch,
		MarketScanSortSearch,
		SymbolFilterSearch {}

const sortColumns = new Set<MarketScanSortColumn>([
	marketScanColumnKeys.dailyRange,
	marketScanColumnKeys.hourlyRange,
	marketScanColumnKeys.dailyRsi14,
	marketScanColumnKeys.weeklyRsi14,
	marketScanColumnKeys.marketCap,
	marketScanColumnKeys.sevenDayChangePercent,
]);

export function parseMarketScanSortSearch(
	search: Record<string, unknown>,
): MarketScanSortSearch {
	const sortColumn = search.sort_column;
	const sortDirection = search.sort_direction;
	if (
		typeof sortColumn === "string" &&
		sortColumns.has(sortColumn as MarketScanSortColumn) &&
		(sortDirection === "asc" || sortDirection === "desc")
	) {
		return {
			sort_column: sortColumn as MarketScanSortColumn,
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
): MarketScanSort {
	return search.sort_column && search.sort_direction
		? { column: search.sort_column, direction: search.sort_direction }
		: defaultMarketScanSort;
}
