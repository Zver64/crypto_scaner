import type { MarketTable, TableRow } from "@/api/generated/models";

export type MarketScanSortDirection = "asc" | "desc";

export interface MarketScanSort {
	column: string;
	direction: MarketScanSortDirection;
}

// A requested column the backend no longer marks sortable falls back to the
// backend default.
export function resolveTableSort(
	table: MarketTable,
	sort: MarketScanSort | undefined,
): MarketScanSort {
	return sort &&
		table.columns.some(({ id, sortable }) => sortable && id === sort.column)
		? sort
		: table.default_sort;
}

export function nextMarketScanSort(
	current: MarketScanSort,
	column: string,
): MarketScanSort {
	if (column !== current.column) {
		return { column, direction: "desc" };
	}

	return {
		column,
		direction: current.direction === "desc" ? "asc" : "desc",
	};
}

export function sortTableRows(
	rows: readonly TableRow[],
	sort: MarketScanSort,
): TableRow[] {
	return [...rows].sort((left, right) => {
		const leftValue = left.cells[sort.column]?.value ?? null;
		const rightValue = right.cells[sort.column]?.value ?? null;
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
