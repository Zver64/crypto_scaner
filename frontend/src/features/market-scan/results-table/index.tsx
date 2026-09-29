import { UnstyledButton } from "@mantine/core";
import { useNavigate } from "@tanstack/react-router";
import type {
	MarketTable,
	PriceHistoryWindow,
	TableRow,
} from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import type { MarketScanCriteria } from "@/features/market-scan/pipeline";
import {
	type CellRenderer,
	cellRenderers,
} from "@/features/market-scan/results-table/cells";
import {
	type MarketScanSort,
	nextMarketScanSort,
	resolveTableSort,
	sortTableRows,
} from "@/features/market-scan/sort";
import { scanCriteriaToSearch } from "@/routes/-scan-criteria-search";

interface MarketScanResultsTableProps {
	criteria: MarketScanCriteria;
	table: MarketTable;
	rows: readonly TableRow[];
	window?: PriceHistoryWindow;
	sort: MarketScanSort | undefined;
	onSortChange(sort: MarketScanSort): void;
}

// Columns, their order, rendering kinds, and sortability come from the backend
// table. Only the first column's position is fixed here: DataTable keeps it
// sticky.
export function MarketScanResultsTable({
	criteria,
	table,
	rows,
	window,
	sort: requestedSort,
	onSortChange,
}: MarketScanResultsTableProps) {
	const navigate = useNavigate();
	const sort = resolveTableSort(table, requestedSort);
	const direction = sort.direction === "desc" ? "descending" : "ascending";
	const columns: DataTableColumn<TableRow>[] = table.columns.map(
		(column, index) => {
			const active = column.sortable && column.id === sort.column;
			// A kind newer than this bundle renders as unavailable.
			const render: CellRenderer | undefined = cellRenderers[column.kind];
			return {
				key: column.id,
				textAlign: index === 0 ? "left" : "center",
				cell: (row) =>
					render
						? render({ cell: row.cells[column.id], column, row, window })
						: "—",
				ariaSort: column.sortable ? (active ? direction : "none") : undefined,
				header: column.sortable ? (
					<UnstyledButton
						aria-label={`Sort by ${column.title}${active ? `, currently ${direction}` : ""}`}
						style={{ font: "inherit" }}
						onClick={() => onSortChange(nextMarketScanSort(sort, column.id))}
					>
						{column.title}
						{active ? (sort.direction === "desc" ? " ↓" : " ↑") : null}
					</UnstyledButton>
				) : (
					column.title
				),
			};
		},
	);

	return (
		<DataTable
			columns={columns}
			rows={sortTableRows(rows, sort)}
			getRowKey={(row) => row.symbol}
			onRowClick={(row) => {
				void navigate({
					params: { symbol: row.symbol },
					search: scanCriteriaToSearch(criteria),
					to: "/instruments/$symbol",
				});
			}}
		/>
	);
}
