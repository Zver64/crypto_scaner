import type { ManipulateType } from "dayjs";
import type {
	BacktestTradeExitReason,
	CandleInterval,
} from "@/api/generated/models";
import type { TradeWords } from "@/features/strategy-backtest/types";

// A long strategy buys and sells; a short one shorts and covers.
export const longTradeWords = {
	close: "sell",
	closed: "sold",
	open: "buy",
	opened: "bought",
} as const satisfies TradeWords;
export const shortTradeWords = {
	close: "cover",
	closed: "covered",
	open: "short",
	opened: "shorted",
} as const satisfies TradeWords;

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

// The dayjs format of the period days, shared by the date pickers and the
// evaluated period so both read alike.
export const backtestDateFormat = "DD.MM.YYYY";

// The dayjs unit of one candle of each interval.
export const intervalUnits = {
	"1h": "hour",
	"1d": "day",
	"1w": "week",
	"1M": "month",
} as const satisfies Record<CandleInterval, ManipulateType>;
