import { UnstyledButton } from "@mantine/core";
import { useNavigate } from "@tanstack/react-router";
import type { PriceHistoryWindow } from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { FavoriteToggle } from "@/features/favorites/favorite-toggle";
import type { MarketScanCriteria } from "@/features/market-scan/pipeline";
import { marketScanColumns } from "@/features/market-scan/results-table/columns";
import { marketScanColumnKeys } from "@/features/market-scan/results-table/keys";
import type { MarketScanRow } from "@/features/market-scan/results-table/utils";
import {
	type MarketScanSort,
	nextMarketScanSort,
	sortMarketScanRows,
} from "@/features/market-scan/sort";
import { scanCriteriaToSearch } from "@/routes/-scan-criteria-search";

interface MarketScanResultsTableProps {
	criteria: MarketScanCriteria;
	rows: readonly MarketScanRow[];
	window?: PriceHistoryWindow;
	alertCounts?: ReadonlyMap<string, number>;
	sort: MarketScanSort;
	onSortChange(sort: MarketScanSort): void;
}

export function MarketScanResultsTable({
	criteria,
	rows,
	window,
	alertCounts,
	sort,
	onSortChange,
}: MarketScanResultsTableProps) {
	const navigate = useNavigate();
	const direction = sort.direction === "desc" ? "descending" : "ascending";
	const columns: DataTableColumn<MarketScanRow>[] = marketScanColumns.map(
		(column) => {
			const sortable = "sortable" in column && column.sortable;
			const active = sortable && column.key === sort.column;
			return {
				key: column.key,
				textAlign:
					column.key === marketScanColumnKeys.symbol ? "left" : "center",
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
	columns.push({
		key: "favorite",
		header: "Favorite",
		textAlign: "center",
		cell: (row) => <FavoriteToggle symbol={row.symbol} />,
	});
	if (alertCounts) {
		columns.push({
			key: "alertCount",
			header: "Alerts",
			textAlign: "center",
			cell: (row) => alertCounts.get(row.symbol) ?? 0,
		});
	}

	return (
		<DataTable
			columns={columns}
			rows={sortMarketScanRows(rows, sort)}
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
