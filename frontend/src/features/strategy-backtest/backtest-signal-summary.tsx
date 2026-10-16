import type { BacktestSignal } from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { BacktestSimpleSignalSummary } from "@/features/strategy-backtest/backtest-simple-signal-summary";
import type { SignalSummaryRow } from "@/features/strategy-backtest/types";
import {
	formatFractionPercent,
	formatSignalSuccesses,
	signalSummaryRows,
	successColor,
} from "@/features/strategy-backtest/utils";
import { formatNumber } from "@/utils/number-format";

interface BacktestSignalSummaryProps {
	signal: BacktestSignal;
}

const columns: DataTableColumn<SignalSummaryRow>[] = [
	{ cell: ({ label }) => label, header: "", key: "label" },
	{
		cell: ({ count }) => count,
		header: "Count (all / counted)",
		key: "count",
		textAlign: "right",
	},
	{
		cell: ({ stats }) => formatFractionPercent(stats.median_move),
		header: "Median move to target",
		key: "move",
		textAlign: "right",
	},
	{
		cell: ({ key, stats }) => {
			const share = key === "signals" ? stats.success_rate : null;
			return (
				<span
					style={{ color: share === null ? undefined : successColor(share) }}
				>
					{formatSignalSuccesses(stats)}
				</span>
			);
		},
		header: "Successful",
		key: "successful",
		textAlign: "right",
	},
	{
		cell: ({ stats }) => formatNumber(stats.neutrals),
		header: "Neutral",
		key: "neutral",
		textAlign: "right",
	},
];

// How often the counted signals succeeded beside every candle; how often a
// simple signal fired.
export function BacktestSignalSummary({ signal }: BacktestSignalSummaryProps) {
	if (signal.evaluation === null) {
		return <BacktestSimpleSignalSummary signal={signal} />;
	}
	return (
		<DataTable
			columns={columns}
			getRowKey={({ key }) => key}
			rows={signalSummaryRows(signal.evaluation)}
		/>
	);
}
