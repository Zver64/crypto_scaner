import type { Field, RuleGroupType, RuleType } from "react-querybuilder";

import type {
	CandleInterval,
	Direction,
	Strategy,
	StrategyCoinSource,
	StrategyIndicatorCall,
	StrategyID,
} from "@/api/generated/models";
import type { PriceSetup } from "@/features/strategy-settings/strategy-price-input/types";
import type { useStrategyForm } from "@/features/strategy-settings/use-strategy-form";

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
	// Text Telegram alerts show under the title; empty shows none.
	message: string;
	query: StrategyQuery;
	// The exit rule; undefined without one.
	exitQuery: StrategyQuery | undefined;
	takeProfit: PriceSetup;
	stopLoss: PriceSetup;
	// The coins chosen, among all coins, which coinSource selected evaluates;
	// a signal's favorites or all coins evaluate other coins instead.
	coins: string[];
	coinSource: StrategyCoinSource;
	// A signal's market cap bounds in millions of USD, a string while being typed;
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

// The fields holding a rule or price expression tree.
export type StrategyTreeField = Extract<
	StrategyFormField,
	"exitQuery" | "query" | "stopLoss" | "takeProfit"
>;

// The strategy form's values as TanStack Form holds them: the trees are
// unknown, since its deep field paths cannot be computed for recursive types
// (TS2589, https://github.com/TanStack/form/issues/1553). Read them through
// strategyFormValues. Once the fix
// (https://github.com/TanStack/form/pull/2422) is released, use
// StrategyFormValues itself and drop this type and strategyFormValues.
export type StrategyFormData = Omit<StrategyFormValues, StrategyTreeField> &
	Record<StrategyTreeField, unknown>;

// The form that holds a draft's fields.
export type StrategyFormApi = ReturnType<typeof useStrategyForm>;

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
	id: StrategyID | undefined;
	// Tells drafts opened one after another apart, so each opens a fresh
	// form and a finished save closes only the draft it saved.
	revision: number;
	// Whether the strategy only signals.
	signal: boolean;
	// Whether the builder dropped parts of a stored rule it cannot show,
	// which saving would remove.
	incomplete: boolean;
	// The indicators the opened rules read, configured or not, which the
	// builder offers besides the configured ones.
	indicators: StrategyIndicatorCall[];
	// The fields the form opens with.
	values: StrategyFormValues;
}

// A value an operand reads, without its period: an indicator output or a
// candle field, which a call reads on a period, such as rsi("1h") or
// bbands_upperband("1h", 20, 2.5), or a position variable of exit rules,
// which has no period.
export interface StrategyVariable {
	// The call without its period, such as bbands_upperband(20, 2.5), or the
	// name of a position variable; tells the variables apart.
	key: string;
	label: string;
	// The function reading an indicator output, such as rsi or
	// bbands_upperband, the candle field, or the position variable.
	name: string;
	// The parameters as the canonical call spells them after the period.
	parameters: string[];
	kind: StrategyVariableKind;
}

export type StrategyVariableKind = "candle" | "indicator" | "position";

// What operands of one rule builder read: the variables offered and the
// periods every call takes.
export interface StrategyVariables {
	intervals: CandleInterval[];
	variables: StrategyVariable[];
}

// The variables one rule builder offers, with the builder's field, the call
// a new condition compares.
export interface StrategyRuleVariables extends StrategyVariables {
	fields: Field[];
}

// The rules of a strategy as the builder opens them: spelled canonically,
// with the indicators they read.
export interface CanonicalRules {
	expressions: Pick<
		Strategy,
		| "exit_expression"
		| "expression"
		| "stop_loss_expression"
		| "take_profit_expression"
	>;
	indicators: StrategyIndicatorCall[];
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
