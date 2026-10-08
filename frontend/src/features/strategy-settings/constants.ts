import { SignalDirection } from "@/api/generated/models";

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

// Names of the moves signals expect.
export const signalDirectionLabels = {
	long: "Long",
	short: "Short",
	sideways: "Sideways",
} as const satisfies Record<SignalDirection, string>;

// The direction a signal starts with when a strategy becomes one.
export const defaultSignalDirection = SignalDirection.long;
