import {
	type BacktestSignalOccurrence,
	Direction,
} from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { BacktestSignalSuccess } from "@/features/strategy-backtest/backtest-signal-success";
import { formatFractionPercent } from "@/features/strategy-backtest/utils";
import { formatDateTime } from "@/utils/date-time-format";
import { formatNumber } from "@/utils/number-format";

interface BacktestSignalTableProps {
	direction: Direction;
	occurrences: readonly BacktestSignalOccurrence[];
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
	{
		cell: ({ counted }) => (counted === null ? "" : counted ? "Yes" : "No"),
		header: "Counted",
		key: "counted",
	},
];

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
		cell: ({ success }) => <BacktestSignalSuccess success={success} />,
		header: "Success",
		key: "success",
	},
];

// The signals, newest first, with their stop and target and whether they
// succeeded; a sideways signal has a target on both sides and no stop.
export function BacktestSignalTable({
	direction,
	occurrences,
}: BacktestSignalTableProps) {
	return (
		<DataTable
			columns={[
				...leadingColumns,
				...(direction === Direction.sideways ? sidewaysColumns : levelColumns),
				...trailingColumns,
			]}
			getRowKey={({ time }) => time}
			rows={[...occurrences].reverse()}
		/>
	);
}
