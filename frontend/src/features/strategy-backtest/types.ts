import type { BacktestTradeStats } from "@/api/generated/models";

// One average of the strategy's trades and of the Every candle baseline.
export interface SummaryRow {
	everyCandle: number | null;
	hint: string;
	key: keyof Pick<
		BacktestTradeStats,
		"average_loss" | "average_trade" | "average_win"
	>;
	label: string;
	strategy: number | null;
}
