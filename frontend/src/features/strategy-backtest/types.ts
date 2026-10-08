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

// The moves of one signal window, after the signals or after every candle.
export interface SignalWindowRow {
	// The window's candle count, shown on its first row only.
	candles: number | undefined;
	key: string;
	label: string;
	// Whether the row measures the signals rather than every candle.
	signals: boolean;
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
