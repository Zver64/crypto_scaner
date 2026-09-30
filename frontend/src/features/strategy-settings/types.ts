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
	query: StrategyQuery;
}
