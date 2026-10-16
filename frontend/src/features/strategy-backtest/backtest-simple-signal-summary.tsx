import type { BacktestSignal } from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import type { SimpleSignalSummaryRow } from "@/features/strategy-backtest/types";
import { simpleSignalSummaryRows } from "@/features/strategy-backtest/utils";

interface BacktestSimpleSignalSummaryProps {
	signal: BacktestSignal;
}

const columns: DataTableColumn<SimpleSignalSummaryRow>[] = [
	{ cell: ({ label }) => label, header: "", key: "label" },
	{
		cell: ({ count }) => count,
		header: "Count",
		key: "count",
		textAlign: "right",
	},
];

// How often a simple signal, which is never judged, fired.
export function BacktestSimpleSignalSummary({
	signal,
}: BacktestSimpleSignalSummaryProps) {
	return (
		<DataTable
			columns={columns}
			getRowKey={({ key }) => key}
			rows={simpleSignalSummaryRows(signal)}
		/>
	);
}
