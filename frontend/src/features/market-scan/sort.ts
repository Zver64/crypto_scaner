import type { MarketScanSortColumn } from "@/features/market-scan/results-table/columns";
import { marketScanColumnKeys } from "@/features/market-scan/results-table/keys";
import type { MarketScanRow } from "@/features/market-scan/results-table/utils";

export type MarketScanSortDirection = "asc" | "desc";

export interface MarketScanSort {
	column: MarketScanSortColumn;
	direction: MarketScanSortDirection;
}

export const defaultMarketScanSort: MarketScanSort = {
	column: marketScanColumnKeys.marketCap,
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

export function sortMarketScanRows(
	rows: readonly MarketScanRow[],
	sort: MarketScanSort,
): MarketScanRow[] {
	return [...rows].sort((left, right) => {
		const leftValue = left[sort.column];
		const rightValue = right[sort.column];
		// Unavailable values are not zero, and remain last in either direction.
		if (leftValue === null && rightValue !== null) return 1;
		if (leftValue !== null && rightValue === null) return -1;
		const comparison =
			leftValue === null || rightValue === null
				? 0
				: sort.direction === "desc"
					? rightValue - leftValue
					: leftValue - rightValue;
		return comparison || left.symbol.localeCompare(right.symbol, "en-US");
	});
}
