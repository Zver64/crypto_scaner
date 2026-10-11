import type { StrategyBacktest } from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import type { TradingSummaryRow } from "@/features/strategy-backtest/types";
import { tradingSummaryRows } from "@/features/strategy-backtest/utils";

const columns: DataTableColumn<TradingSummaryRow>[] = [
	{ key: "label", header: "Metric", cell: (row) => row.label },
	{
		key: "value",
		header: "Strategy",
		cell: (row) => row.value,
		textAlign: "right",
	},
	{ key: "compared", header: "Compared with", cell: (row) => row.compared },
];

export function BacktestTradingSummary({
	backtest,
}: {
	backtest: StrategyBacktest;
}) {
	return (
		<DataTable
			columns={columns}
			getRowKey={(row) => row.label}
			rows={tradingSummaryRows(backtest)}
		/>
	);
}
