import type { Field, RuleGroupType, RuleType } from "react-querybuilder";
import type { Direction, StrategyVariable } from "@/api/generated/models";
import type { PriceSetup } from "@/features/strategy-settings/strategy-price-input/types";

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

// The editable fields of a strategy form.
export interface StrategyFormValues {
	name: string;
	// How the strategy trades or the move the signal expects.
	direction: Direction;
	// Telegram alert text; empty keeps the generated one.
	message: string;
	query: StrategyQuery;
	// The exit rule; undefined without one.
	exitQuery: StrategyQuery | undefined;
	takeProfit: PriceSetup;
	stopLoss: PriceSetup;
	// The market cap bounds in millions of USD, a string while being typed;
	// empty without a bound.
	minMarketCap: number | string;
	maxMarketCap: number | string;
	// How backtests judge a signal: the target distance in stops and the
	// candles after each signal, as the selects emit them; a trading strategy
	// ignores them.
	targetRatio: string;
	window: string;
}

export type StrategyFormField = keyof StrategyFormValues;

// The fields holding a rule builder's query.
export type StrategyRuleField = Extract<
	StrategyFormField,
	"exitQuery" | "query"
>;

// The fields holding a take profit or a stop loss.
export type StrategyPriceField = Extract<
	StrategyFormField,
	"stopLoss" | "takeProfit"
>;

export interface StrategyDraft {
	// Undefined while creating a strategy.
	id: number | undefined;
	// Tells drafts opened one after another apart, so each opens a fresh
	// form and a finished save closes only the draft it saved.
	revision: number;
	// Whether the strategy only signals.
	signal: boolean;
	// Whether the builder dropped parts of a stored rule it cannot show,
	// which saving would remove.
	incomplete: boolean;
	// The fields the form opens with.
	values: StrategyFormValues;
}

// The variables one rule builder offers, with the builder fields made of
// them.
export interface StrategyRuleVariables {
	fields: Field[];
	variables: StrategyVariable[];
}

// The variables of the entry rule and prices, which exist at every candle,
// and those of the exit rule, which may also read the open position.
export interface StrategyVariableSets {
	entry: StrategyRuleVariables;
	exit: StrategyRuleVariables;
}

// A token of a strategy expression and its position in the source.
export interface Token {
	text: string;
	start: number;
	end: number;
}

// Whether a strategy only signals or trades, fixed when it is created.
export type StrategyKind = "signal" | "strategy";
