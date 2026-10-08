import type { BacktestTradeStats } from "@/api/generated/models";

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
