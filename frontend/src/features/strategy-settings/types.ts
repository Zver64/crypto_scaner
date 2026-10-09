import type { RuleGroupType, RuleType } from "react-querybuilder";
import type {
	Direction,
	SignalTargetRatio,
	SignalWindow,
} from "@/api/generated/models";

// Comparison operators of the strategy language. Range operators take two
// numbers; crosses compare the latest and the previous closed candle.
export type StrategyOperator =
	| ">"
	| ">="
	| "<"
	| "<="
	| "between"
	| "notBetween"
	| "crossesAbove"
	| "crossesBelow";

export type StrategyRule = RuleType<string, StrategyOperator>;

export type StrategyQuery = RuleGroupType<StrategyRule>;

export interface StrategyDraft {
	// Undefined while creating a strategy.
	id: number | undefined;
	// Tells drafts opened one after another apart, so each opens a fresh
	// form and a finished save closes only the draft it saved.
	revision: number;
	name: string;
	// Whether the strategy only signals.
	signal: boolean;
	// How the strategy trades or the move the signal expects.
	direction: Direction;
	// Telegram alert text; empty keeps the generated one.
	message: string;
	query: StrategyQuery;
	// The exit rule; undefined without one.
	exitQuery: StrategyQuery | undefined;
	// The take profit and stop loss price expressions; empty without them.
	takeProfit: string;
	stopLoss: string;
	// The market cap bounds in millions of USD; empty without a bound.
	minMarketCap: number | "";
	maxMarketCap: number | "";
	// How backtests judge a signal: the target distance in stops and the
	// candles after each signal; a trading strategy ignores them.
	targetRatio: SignalTargetRatio;
	window: SignalWindow;
	// Whether the builder dropped parts of a stored rule it cannot show,
	// which saving would remove.
	incomplete: boolean;
}

// A token of a strategy expression and its position in the source.
export interface Token {
	text: string;
	start: number;
	end: number;
}

// Whether a strategy only signals or trades, fixed when it is created.
export type StrategyKind = "signal" | "strategy";
