import type { BacktestTradeExitReason } from "@/api/generated/models";

// Meanings of the backtest baselines, shown on hover or tap.
export const buyAndHoldHint =
	"Bought at the start of the period and sold at its end.";
export const dcaHint =
	"The same amount bought on every candle and valued at the end of the period.";

// Short names of what sold a trade.
export const exitReasonLabels = {
	exit: "Rule",
	stop_loss: "SL",
	take_profit: "TP",
} as const satisfies Record<BacktestTradeExitReason, string>;
