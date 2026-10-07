import { Alert, Badge, Group, Stack, Text } from "@mantine/core";
import type {
	CandleHistoryLoad,
	CandleHistoryLoadJob,
} from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { intervalLabel } from "@/features/command-settings/utils";
import { formatDateTime } from "@/utils/date-time-format";
import { formatNumber } from "@/utils/number-format";

interface HistoryLoadJobProps {
	job: CandleHistoryLoadJob;
}

const statusBadges = {
	done: { color: "green", label: "Done" },
	failed: { color: "red", label: "Failed" },
	running: { color: "blue", label: "Running" },
} as const;

const columns: DataTableColumn<CandleHistoryLoad>[] = [
	{ cell: (item) => item.symbol, header: "Coin", key: "symbol" },
	{
		cell: (item) => intervalLabel(item.interval),
		header: "Interval",
		key: "interval",
	},
	{
		cell: (item) =>
			item.not_ready ? (
				<Text c="dimmed" inherit>
					No candles yet, skipped
				</Text>
			) : (
				formatNumber(item.count)
			),
		header: "Candles",
		key: "count",
		textAlign: "right",
	},
	{
		cell: (item) =>
			item.oldest_open_time ? formatDateTime(item.oldest_open_time) : "—",
		header: "Oldest",
		key: "oldest",
	},
	{
		cell: (item) =>
			item.not_ready
				? "—"
				: item.exhausted
					? "All Binance has"
					: "More available",
		header: "Binance",
		key: "exhausted",
	},
];

// The latest history load: its request, its state, and the stored history of
// every coin and interval loaded so far; pairs without synchronized candles
// yet are skipped.
export function HistoryLoadJob({ job }: HistoryLoadJobProps) {
	const badge = statusBadges[job.status];
	return (
		<Stack gap="xs">
			<Group gap="xs">
				<Text fw={600} size="sm">
					Latest load
				</Text>
				<Badge color={badge.color} size="sm" variant="light">
					{badge.label}
				</Badge>
			</Group>
			<Text c="dimmed" size="xs">
				{job.symbols.join(", ")} · {job.intervals.map(intervalLabel).join(", ")}{" "}
				· depth {formatNumber(job.depth)} · started{" "}
				{formatDateTime(job.started_at)}
				{job.finished_at
					? ` · finished ${formatDateTime(job.finished_at)}`
					: ""}
			</Text>
			{job.status === "failed" ? (
				<Alert color="red" variant="light">
					{job.error ?? "The history load failed."}
					{job.history_changed
						? " Some candles were stored before the failure."
						: ""}
				</Alert>
			) : null}
			{job.items.length > 0 ? (
				<DataTable
					columns={columns}
					getRowKey={(item) => `${item.symbol}:${item.interval}`}
					rows={job.items}
				/>
			) : null}
		</Stack>
	);
}
