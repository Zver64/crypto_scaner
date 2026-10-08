import type {
	BacktestSignalOccurrence,
	BacktestSignalWindow,
	Direction,
} from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { BacktestReturn } from "@/features/strategy-backtest/backtest-return";
import { signalChangeColors } from "@/features/strategy-backtest/constants";
import { formatDateTime } from "@/utils/date-time-format";
import { formatNumber } from "@/utils/number-format";

interface BacktestSignalTableProps {
	direction: Direction;
	occurrences: readonly BacktestSignalOccurrence[];
	windows: readonly BacktestSignalWindow[];
}

const columns: DataTableColumn<BacktestSignalOccurrence>[] = [
	{
		cell: ({ time }) => formatDateTime(time),
		header: "Candle",
		key: "time",
	},
	{
		cell: ({ close }) => formatNumber(close),
		header: "Close",
		key: "close",
		textAlign: "right",
	},
];

// The changes of the close in the order of the signal windows, colored by
// whether they moved as direction expected.
function changeColumns(
	direction: Direction,
	windows: readonly BacktestSignalWindow[],
): DataTableColumn<BacktestSignalOccurrence>[] {
	return windows.map(({ candles }, index) => ({
		cell: ({ changes }) => (
			<BacktestReturn
				colors={signalChangeColors[direction]}
				value={changes[index]?.change ?? null}
			/>
		),
		header: `+${formatNumber(candles)}`,
		key: `change-${candles}`,
		textAlign: "right",
	}));
}

// The signals, newest first, with the close of their candle and its change
// over each window.
export function BacktestSignalTable({
	direction,
	occurrences,
	windows,
}: BacktestSignalTableProps) {
	return (
		<DataTable
			columns={[...columns, ...changeColumns(direction, windows)]}
			getRowKey={({ time }) => time}
			rows={[...occurrences].reverse()}
		/>
	);
}
