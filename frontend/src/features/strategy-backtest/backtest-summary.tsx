import type { StrategyBacktest } from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { BacktestReturn } from "@/features/strategy-backtest/backtest-return";
import { HintLabel } from "@/features/strategy-backtest/hint-label";
import type { SummaryRow } from "@/features/strategy-backtest/types";

interface BacktestSummaryProps {
	backtest: StrategyBacktest;
}

const averages = [
	{
		hint: "Average net return of closed trades.",
		key: "average_trade",
		label: "Avg trade",
	},
	{
		hint: "Average net return of winning trades.",
		key: "average_win",
		label: "Avg win",
	},
	{
		hint: "Average net return of losing trades.",
		key: "average_loss",
		label: "Avg loss",
	},
] as const satisfies readonly Omit<SummaryRow, "strategy">[];

const columns: DataTableColumn<SummaryRow>[] = [
	{
		cell: (row) => <HintLabel hint={row.hint}>{row.label}</HintLabel>,
		header: "",
		key: "label",
	},
	{
		cell: (row) => <BacktestReturn value={row.strategy} />,
		header: "Strategy",
		key: "strategy",
		textAlign: "right",
	},
];

// The average returns of the closed trades, which the metric cards leave
// out.
export function BacktestSummary({
	backtest: { summary },
}: BacktestSummaryProps) {
	const rows = averages.map(
		(average): SummaryRow => ({
			...average,
			strategy: summary.stats[average.key],
		}),
	);
	return (
		<DataTable columns={columns} getRowKey={(row) => row.key} rows={rows} />
	);
}
