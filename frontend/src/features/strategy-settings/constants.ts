import { Direction } from "@/api/generated/models";
import type { StrategyKind } from "@/features/strategy-settings/types";

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

// Lets the expression parser accept every field; the backend validates them.
export const anyField = { fieldExists: () => true };

// Names of the directions strategies trade and the moves signals expect.
export const directionLabels = {
	long: "Long",
	short: "Short",
	sideways: "Sideways",
} as const satisfies Record<Direction, string>;

// The direction a new strategy or signal starts with.
export const defaultDirection = Direction.long;

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

// The directions a strategy trades; only a signal expects a sideways move.
export const strategyDirections = [Direction.long, Direction.short] as const;
