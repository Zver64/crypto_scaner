import type {
	BacktestSignalStats,
	BacktestTradeStats,
} from "@/api/generated/models";

// One average of the strategy's closed trades.
export interface SummaryRow {
	hint: string;
	key: keyof Pick<
		BacktestTradeStats,
		"average_loss" | "average_trade" | "average_win"
	>;
	label: string;
	strategy: number | null;
}

// One row of a signal's backtest summary: the counted signals or every
// candle.
export interface SignalSummaryRow {
	count: string;
	key: "all" | "signals";
	label: string;
	stats: BacktestSignalStats;
}

// What a strategy does when it opens and closes a trade, as its backtest
// names it.
export interface TradeWords {
	open: string;
	opened: string;
	close: string;
	closed: string;
}
