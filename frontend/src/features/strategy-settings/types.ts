import type { RuleGroupType, RuleType } from "react-querybuilder";

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
	name: string;
	// Telegram alert text; empty keeps the generated one.
	message: string;
	query: StrategyQuery;
	// Whether the builder dropped parts of the stored expression it cannot
	// show, which saving would remove.
	incomplete: boolean;
}

// A token of a strategy expression and its position in the source.
export interface Token {
	text: string;
	start: number;
	end: number;
}
