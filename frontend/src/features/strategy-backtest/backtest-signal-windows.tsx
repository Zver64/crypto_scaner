import type { BacktestSignalWindow } from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { BacktestReturn } from "@/features/strategy-backtest/backtest-return";
import type { SignalWindowRow } from "@/features/strategy-backtest/types";
import {
	formatFractionPercent,
	hitsColor,
	signalWindowRows,
} from "@/features/strategy-backtest/utils";
import { formatNumber } from "@/utils/number-format";

interface BacktestSignalWindowsProps {
	windows: readonly BacktestSignalWindow[];
}

const columns: DataTableColumn<SignalWindowRow>[] = [
	{
		cell: ({ candles }) => (candles === undefined ? "" : formatNumber(candles)),
		header: "Candles",
		key: "candles",
		textAlign: "right",
	},
	{ cell: ({ label }) => label, header: "After", key: "after" },
	{
		cell: ({ stats }) => formatNumber(stats.count),
		header: "Count",
		key: "count",
		textAlign: "right",
	},
	{
		cell: ({ stats }) => <BacktestReturn value={stats.rise} />,
		header: "Rise",
		key: "rise",
		textAlign: "right",
	},
	{
		cell: ({ stats }) => <BacktestReturn value={stats.fall} />,
		header: "Fall",
		key: "fall",
		textAlign: "right",
	},
	{
		cell: ({ stats }) => formatFractionPercent(stats.range),
		header: "Range",
		key: "range",
		textAlign: "right",
	},
	{
		cell: ({ signals, stats }) => (
			<span
				style={{
					color:
						signals && stats.hits !== null ? hitsColor(stats.hits) : undefined,
				}}
			>
				{formatFractionPercent(stats.hits)}
			</span>
		),
		header: "Hits",
		key: "hits",
		textAlign: "right",
	},
];

// The median moves over each window after the signals beside those after
// every evaluated candle; the hits of the signals are colored from red to
// green.
export function BacktestSignalWindows({ windows }: BacktestSignalWindowsProps) {
	return (
		<DataTable
			columns={columns}
			getRowKey={({ key }) => key}
			rows={signalWindowRows(windows)}
		/>
	);
}
