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

export interface MarketScanSearch extends ScanCriteriaSearch {
	sort_column?: MarketScanSortColumn;
	sort_direction?: MarketScanSortDirection;
	symbol_filter?: string;
}

const sortColumns = new Set<MarketScanSortColumn>([
	marketScanColumnKeys.dailyRange,
	marketScanColumnKeys.hourlyRange,
	marketScanColumnKeys.marketCap,
	marketScanColumnKeys.sevenDayChangePercent,
]);

export function parseMarketScanSearch(
	search: Record<string, unknown>,
): MarketScanSearch {
	const parsed: MarketScanSearch = parseOptionalScanCriteriaSearch(search);
	const symbolFilter = search.symbol_filter;
	if (typeof symbolFilter === "string" && symbolFilter.length > 0) {
		parsed.symbol_filter = symbolFilter;
	}

	const sortColumn = search.sort_column;
	const sortDirection = search.sort_direction;
	if (
		typeof sortColumn === "string" &&
		sortColumns.has(sortColumn as MarketScanSortColumn) &&
		(sortDirection === "asc" || sortDirection === "desc")
	) {
		parsed.sort_column = sortColumn as MarketScanSortColumn;
		parsed.sort_direction = sortDirection;
	}

	return parsed;
}

export function marketScanSortFromSearch(
	search: MarketScanSearch,
): MarketScanSort {
	return search.sort_column && search.sort_direction
		? { column: search.sort_column, direction: search.sort_direction }
		: defaultMarketScanSort;
}
