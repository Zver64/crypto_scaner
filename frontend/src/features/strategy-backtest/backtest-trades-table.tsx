import type { BacktestTrade, StrategyBacktest } from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { BacktestReturn } from "@/features/strategy-backtest/backtest-return";
import { formatDateTime } from "@/utils/date-time-format";
import { formatNumber } from "@/utils/number-format";

interface BacktestTradesTableProps {
	backtest: StrategyBacktest;
}

// Trades span years of history, so their times carry the year.
const columns: DataTableColumn<BacktestTrade>[] = [
	{
		cell: (trade) => formatDateTime(trade.entry_time),
		header: "Entry",
		key: "entry-time",
	},
	{
		cell: (trade) => formatNumber(trade.entry_price),
		header: "Entry price",
		key: "entry-price",
		textAlign: "right",
	},
	{
		cell: (trade) => formatDateTime(trade.exit_time),
		header: "Exit",
		key: "exit-time",
	},
	{
		cell: (trade) => formatNumber(trade.exit_price),
		header: "Exit price",
		key: "exit-price",
		textAlign: "right",
	},
	{
		cell: (trade) => <BacktestReturn value={trade.net_return} />,
		header: "Net return",
		key: "net-return",
		textAlign: "right",
	},
];

// Closed trades, newest first. Entries buy at the open of the entry candle,
// exits sell at the close of the exit candle.
export function BacktestTradesTable({
	backtest: { trades },
}: BacktestTradesTableProps) {
	const rows = [...trades].reverse();
	return (
		<DataTable
			columns={columns}
			getRowKey={(trade) => trade.entry_time}
			rows={rows}
		/>
	);
}
