import type { Translations } from "react-querybuilder";

import {
	Direction,
	SignalTargetRatio,
	SignalWindow,
} from "@/api/generated/models";
import type {
	StrategyKind,
	StrategyOperator,
	StrategyVariableKind,
} from "@/features/strategy-settings/types";

// Tokens of a strategy expression, for parenthesizeOperands.
export const tokenPattern =
	/\s+|"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|\.\d+|[A-Za-z_]\w*|&&|\|\||[<>=!]=|./g;
export const comparisonTokens = [">", ">=", "<", "<=", "==", "!="];
export const arithmeticTokens = ["+", "-", "*", "/", "%"];
// Tokens that end an operand of a comparison at its own depth.
export const operandBoundaries = [
	"&&",
	"||",
	",",
	"?",
	":",
	...comparisonTokens,
];

// The comparison operators the rule builder offers.
export const strategyOperators: { label: string; name: StrategyOperator }[] = [
	{ label: "> above", name: ">" },
	{ label: "≥ at least", name: ">=" },
	{ label: "< below", name: "<" },
	{ label: "≤ at most", name: "<=" },
	{ label: "between", name: "between" },
	{ label: "not between", name: "notBetween" },
	{ label: "crosses above", name: "crossesAbove" },
	{ label: "crosses below", name: "crossesBelow" },
];

// How a group combines its conditions.
export const strategyCombinators = [
	{ label: "AND", name: "and" },
	{ label: "OR", name: "or" },
];

// The labels of the rule builder's group header.
export const ruleBuilderTranslations = {
	addGroup: { label: "+ Group" },
	addRule: { label: "+ Condition" },
	notToggle: { label: "NOT" },
	removeGroup: { title: "Remove group" },
} satisfies Partial<Translations>;

// The variable select's group of each kind of variable.
export const variableGroups = {
	candle: "Candle",
	indicator: "Indicators",
	position: "Position",
} as const satisfies Record<StrategyVariableKind, string>;

// Lets the expression parser accept every field; the backend validates them.
export const anyField = { fieldExists: () => true };

// Names of the directions strategies trade and the moves signals expect.
export const directionLabels = {
	simple: "Simple",
	long: "Long",
	short: "Short",
	sideways: "Sideways",
} as const satisfies Record<Direction, string>;

// The direction a new strategy or signal starts with.
export const defaultDirections = {
	signal: Direction.simple,
	strategy: Direction.long,
} as const satisfies Record<StrategyKind, Direction>;

// The target ratio and window a new signal starts with.
export const defaultSignalTargetRatio = SignalTargetRatio.NUMBER_2;
export const defaultSignalWindow = SignalWindow.NUMBER_6;

// The target ratios and windows a signal may have.
export const signalTargetRatios = Object.values(SignalTargetRatio);
export const signalWindows = Object.values(SignalWindow);

// How each kind is named in its page and form.
export const kindNouns = {
	signal: { name: "Signal", one: "signal", many: "signals", title: "Signals" },
	strategy: {
		name: "Strategy",
		one: "strategy",
		many: "strategies",
		title: "Strategies",
	},
} as const satisfies Record<
	StrategyKind,
	{ name: string; one: string; many: string; title: string }
>;

// The directions a strategy trades; only a signal may be simple or expect a
// sideways move.
export const strategyDirections = [Direction.long, Direction.short] as const;
