export type MarketScanSortColumn =
	| "daily_volatility"
	| "hourly_volatility"
	| "market_cap";
export type MarketScanSortDirection = "asc" | "desc";

export interface MarketScanSort {
	column: MarketScanSortColumn;
	direction: MarketScanSortDirection;
}

export const defaultMarketScanSort: MarketScanSort = {
	column: "daily_volatility",
	direction: "desc",
};

export function nextMarketScanSort(
	current: MarketScanSort,
	column: MarketScanSortColumn,
): MarketScanSort {
	if (column !== current.column) {
		return { column, direction: "desc" };
	}

	return {
		column,
		direction: current.direction === "desc" ? "asc" : "desc",
	};
}
