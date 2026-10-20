import type {
	BacktestSignalStats,
	BacktestTradeStats,
	StrategyBacktest,
	StrategyID,
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

// One row of a simple signal's backtest summary: its signals or the
// candles it could fire on.
export interface SimpleSignalSummaryRow {
	count: string;
	key: "candles" | "signals";
	label: string;
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

export interface TradingSummaryRow {
	label: string;
	value: string;
	compared: string;
}

export interface BacktestRunParameters {
	strategy: StrategyID;
	symbols: string[];
	from: string | undefined;
	to: string | undefined;
}

export interface BacktestCoinResult {
	symbol: string;
	status:
		| "queued"
		| "running"
		| "completed"
		| "failed"
		| "cancelled"
		| "not_run";
	result?: StrategyBacktest;
	error?: string;
}

export interface BacktestRun {
	parameters: BacktestRunParameters;
	items: BacktestCoinResult[];
	revision: number;
	// When the run started; candles closed by then count as stored.
	startedAt: number;
}
