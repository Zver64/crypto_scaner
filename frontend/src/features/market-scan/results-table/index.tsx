import { UnstyledButton } from "@mantine/core";
import type { PriceHistoryWindow } from "@/api/client";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { marketScanColumns } from "@/features/market-scan/results-table/columns";
import type { MarketScanRow } from "@/features/market-scan/results-table/utils";
import {
	type MarketScanSort,
	nextMarketScanSort,
	sortMarketScanRows,
} from "@/features/market-scan/sort";

interface MarketScanResultsTableProps {
	rows: readonly MarketScanRow[];
	window: PriceHistoryWindow;
	sort: MarketScanSort;
	onSortChange(sort: MarketScanSort): void;
	onSelectInstrument(symbol: string): void;
}

export function MarketScanResultsTable({
	rows,
	window,
	sort,
	onSortChange,
	onSelectInstrument,
}: MarketScanResultsTableProps) {
	const direction = sort.direction === "desc" ? "descending" : "ascending";
	const columns: DataTableColumn<MarketScanRow>[] = marketScanColumns.map(
		(column) => {
			const sortable = "sortable" in column && column.sortable;
			const active = sortable && column.key === sort.column;
			return {
				key: column.key,
				textAlign: "textAlign" in column ? column.textAlign : undefined,
				cell: (row) => column.cell(row, window),
				ariaSort: sortable ? (active ? direction : "none") : undefined,
				header: sortable ? (
					<UnstyledButton
						aria-label={`Sort by ${column.header}${active ? `, currently ${direction}` : ""}`}
						style={{ font: "inherit" }}
						onClick={() => onSortChange(nextMarketScanSort(sort, column.key))}
					>
						{column.header}
						{active ? (sort.direction === "desc" ? " ↓" : " ↑") : null}
					</UnstyledButton>
				) : (
					column.header
				),
			};
		},
	);

	return (
		<DataTable
			columns={columns}
			rows={sortMarketScanRows(rows, sort)}
			getRowKey={(row) => row.symbol}
			minWidth={760}
			onRowClick={(row) => onSelectInstrument(row.symbol)}
		/>
	);
}
