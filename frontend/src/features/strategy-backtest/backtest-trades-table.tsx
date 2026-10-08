import {
	type BacktestTrade,
	Direction,
	type StrategyBacktest,
} from "@/api/generated/models";
import { DataTable, type DataTableColumn } from "@/components/data-table";
import { BacktestReturn } from "@/features/strategy-backtest/backtest-return";
import { exitReasonLabels } from "@/features/strategy-backtest/constants";
import { formatPriceLevels } from "@/features/strategy-backtest/utils";
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
		header: "Avg entry",
		key: "entry-price",
		textAlign: "right",
	},
	{
		cell: (trade) => formatNumber(trade.buys),
		header: "Buys",
		key: "buys",
		textAlign: "right",
	},
	{
		cell: (trade) => (trade.open ? "Open" : formatDateTime(trade.exit_time)),
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
		cell: (trade) =>
			trade.exit_reason === undefined
				? "—"
				: exitReasonLabels[trade.exit_reason],
		header: "By",
		key: "exit-reason",
	},
	{
		cell: (trade) => formatPriceLevels(trade.take_profit, trade.stop_loss),
		header: "TP / SL",
		key: "levels",
		textAlign: "right",
	},
	{
		cell: (trade) => <BacktestReturn value={trade.net_return} />,
		header: "Net return",
		key: "net-return",
		textAlign: "right",
	},
];

// Trades, newest first, with what sold them and the take profit and stop
// loss fixed at their entry. Buys fill at the open after their signal and
// are averaged; a take profit or stop loss sells on the candle reaching it,
// an exit rule at the open after its signal. An open trade is valued at the
// last close.
export function BacktestTradesTable({
	backtest: { direction, trades },
}: BacktestTradesTableProps) {
	const rows = [...trades].reverse();
	return (
		<DataTable
			columns={
				// A short strategy holds one short per trade.
				direction === Direction.short
					? columns.filter(({ key }) => key !== "buys")
					: columns
			}
			getRowKey={(trade) => trade.entry_time}
			rows={rows}
		/>
	);
}
