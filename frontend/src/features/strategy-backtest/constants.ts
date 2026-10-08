import type { ManipulateType } from "dayjs";
import type {
	BacktestTradeExitReason,
	CandleInterval,
	SignalDirection,
} from "@/api/generated/models";
import type { PercentChangeColors } from "@/components/percent-change";

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

// The moves a signal of each direction expects, which count as hits.
export const signalHitHints = {
	long: "a rise above the fall",
	short: "a fall deeper than the rise",
	sideways: "a range narrower than the median range after every candle",
} as const satisfies Record<SignalDirection, string>;

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

// How the price changes after a signal are colored: green for the move it
// expected, red against it, and plain for sideways, which expects neither.
export const signalChangeColors = {
	long: "sign",
	short: "inverted",
	sideways: "none",
} as const satisfies Record<SignalDirection, PercentChangeColors>;
