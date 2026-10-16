import {
	type BacktestIndicatorColumn,
	type BacktestSignal,
	type BacktestSignalOccurrence,
	Direction,
} from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { BacktestSignalSuccess } from "@/features/strategy-backtest/backtest-signal-success";
import { formatFractionPercent } from "@/features/strategy-backtest/utils";
import { formatDateTime } from "@/utils/date-time-format";
import { formatNumber } from "@/utils/number-format";

interface BacktestSignalTableProps {
	indicatorColumns: readonly BacktestIndicatorColumn[];
	direction: Direction;
	signal: BacktestSignal;
}

// A distance of an evaluated signal, empty without an evaluation.
function distance(value: number | null) {
	return value === null ? "" : formatFractionPercent(value);
}

const leadingColumns: DataTableColumn<BacktestSignalOccurrence>[] = [
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

const countedColumn: DataTableColumn<BacktestSignalOccurrence> = {
	cell: ({ counted }) => (counted === null ? "" : counted ? "Yes" : "No"),
	header: "Counted",
	key: "counted",
};

const levelColumns: DataTableColumn<BacktestSignalOccurrence>[] = [
	{
		cell: ({ stop }) => distance(stop),
		header: "Stop",
		key: "stop",
		textAlign: "right",
	},
	{
		cell: ({ target }) => distance(target),
		header: "Target",
		key: "target",
		textAlign: "right",
	},
];

const sidewaysColumns: DataTableColumn<BacktestSignalOccurrence>[] = [
	{
		cell: ({ target }) => (target === null ? "" : `±${distance(target)}`),
		header: "Targets",
		key: "targets",
		textAlign: "right",
	},
];

const trailingColumns: DataTableColumn<BacktestSignalOccurrence>[] = [
	{
		cell: ({ move }) => distance(move),
		header: "Move to target",
		key: "move",
		textAlign: "right",
	},
	{
		cell: ({ outcome }) => <BacktestSignalSuccess outcome={outcome} />,
		header: "Success",
		key: "success",
	},
];

// The signals, newest first, with their stop and target and whether they
// succeeded, unless the signal is simple and never judged; a sideways signal
// has a target on both sides and no stop.
export function BacktestSignalTable({
	direction,
	signal,
	indicatorColumns,
}: BacktestSignalTableProps) {
	return (
		<DataTable
			columns={[
				...leadingColumns,
				...(signal.evaluation === null
					? []
					: [
							countedColumn,
							...(direction === Direction.sideways
								? sidewaysColumns
								: levelColumns),
							...trailingColumns,
						]),
				...indicatorColumns.map(
					({ key, title }): DataTableColumn<BacktestSignalOccurrence> => ({
						key: `indicator:${key}`,
						header: title,
						textAlign: "right",
						cell: ({ indicator_values }) =>
							indicator_values[key] === undefined
								? ""
								: formatNumber(indicator_values[key]),
					}),
				),
			]}
			getRowKey={({ time }) => time}
			rows={[...signal.occurrences].reverse()}
		/>
	);
}
