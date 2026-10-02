import {
	defaultCELSerializers,
	type ExpressionFunctionMetaRegistry,
	type ExpressionNode,
	getExpressionParserCEL,
	getExpressionRuleProcessorCEL,
	quoteLeaf,
	type SQLSerializerRegistry,
	serializeInfix,
} from "@react-querybuilder/expr";

// A function of the strategy language: its builder label, its argument
// titles, and the arguments it takes when first chosen. Variadic functions
// accept further arguments titled like the last one; minArgs lets trailing
// arguments be left out.
interface StrategyFunction {
	label: string;
	args: string[];
	defaults: ExpressionNode[];
	variadic?: boolean;
	minArgs?: number;
}

type FunctionNode = Extract<ExpressionNode, { kind: "func" }>;

const number = (value: number): ExpressionNode => ({ kind: "value", value });

// The functions the backend compiles, keyed as the expression nodes name
// them. Arithmetic keeps the package's operator keys.
export const strategyFunctions: Record<string, StrategyFunction> = {
	add: { label: "+ add", args: ["Value", "Added"], defaults: [number(0)] },
	subtract: {
		label: "− subtract",
		args: ["Value", "Subtracted"],
		defaults: [number(0)],
	},
	multiply: {
		label: "× multiply",
		args: ["Value", "Factor"],
		defaults: [number(1)],
	},
	divide: {
		label: "÷ divide",
		args: ["Value", "Divisor"],
		defaults: [number(1)],
	},
	min: {
		label: "min",
		args: ["Value", "Value"],
		defaults: [number(0)],
		variadic: true,
	},
	max: {
		label: "max",
		args: ["Value", "Value"],
		defaults: [number(0)],
		variadic: true,
	},
	abs: { label: "abs", args: ["Value"], defaults: [] },
	mod: { label: "mod", args: ["Value", "Divisor"], defaults: [number(1)] },
	prev: {
		label: "prev — earlier candle",
		args: ["Value", "Candles back"],
		defaults: [number(1)],
		// prev(x) reads one candle back.
		minArgs: 1,
	},
	percentile: {
		label: "percentile — of earlier candles",
		args: ["Value", "Candles", "Percentile"],
		defaults: [number(120), number(20)],
	},
};

// of("BTCUSDT", x) reads the operand x of another coin. It is not a
// function over values, so the builder offers it as an operand kind of its
// own rather than among strategyFunctions.
const coinFunction = "of";

// min and max take up to ten values on the backend.
export const maxVariadicArguments = 10;

// The inclusive range of argument counts a function accepts.
function argumentRange(definition: StrategyFunction): [number, number] {
	return definition.variadic
		? [2, maxVariadicArguments]
		: [definition.minArgs ?? definition.args.length, definition.args.length];
}

const functionMeta: ExpressionFunctionMetaRegistry = {
	...Object.fromEntries(
		Object.entries(strategyFunctions).map(([name, definition]) => [
			name,
			{
				label: definition.label,
				arity: argumentRange(definition),
			},
		]),
	),
	[coinFunction]: { label: "another coin", arity: [2, 2] },
};

// Named functions are written as calls, which the backend compiles and the
// parser reads back.
const callNames = [
	"min",
	"max",
	"abs",
	"mod",
	"prev",
	"percentile",
	coinFunction,
];

const serializers: SQLSerializerRegistry = {
	...defaultCELSerializers,
	...Object.fromEntries(
		callNames.map((name) => [
			name,
			(_options: unknown, ...args: string[]) => `${name}(${args.join(", ")})`,
		]),
	),
};

// Exports rules whose sides hold expressions.
export const expressionRuleProcessor =
	getExpressionRuleProcessorCEL(serializers);

// Reads arithmetic and function operands of stored expressions.
export const expressionParser = getExpressionParserCEL(
	{ functions: Object.fromEntries(callNames.map((name) => [name, name])) },
	functionMeta,
);

// The CEL source of one operand.
export function expressionSource(node: ExpressionNode): string {
	return serializeInfix(node, serializers, {
		renderField: (field) => field,
		// Coins are quoted; numbers stay bare.
		renderLeaf: (leaf, options) => quoteLeaf(leaf, `"`, options),
	});
}

// The node a function call starts with when chosen around node.
export function functionCall(name: string, node: ExpressionNode): FunctionNode {
	return {
		kind: "func",
		fn: name,
		args: [node, ...(strategyFunctions[name]?.defaults ?? [])],
	};
}

// node read from another coin, which is still to be chosen.
export function coinCall(node: ExpressionNode): FunctionNode {
	return { kind: "func", fn: coinFunction, args: [coin(""), node] };
}

// node read from symbol instead of the coin of call.
export function withCoin(call: FunctionNode, symbol: string): FunctionNode {
	return { ...call, args: [coin(symbol), ...call.args.slice(1)] };
}

const coin = (symbol: string): ExpressionNode => ({
	kind: "value",
	value: symbol,
});

// The coin node reads from, empty while it is not chosen, or undefined when
// node reads the evaluated coin.
export function coinOf(node: ExpressionNode): string | undefined {
	if (node.kind !== "func" || node.fn !== coinFunction) return undefined;
	const [symbol] = node.args;
	return symbol?.kind === "value" && typeof symbol.value === "string"
		? symbol.value
		: "";
}

// The call of name that replaces node, keeping the arguments that still fit.
export function replaceFunction(
	node: FunctionNode,
	name: string,
): FunctionNode {
	const call = functionCall(name, node.args[0] ?? number(0));
	return {
		...call,
		args: call.args.map((arg, index) => node.args[index] ?? arg),
	};
}

// Whether every field is chosen, every number finite, and every function
// known with a valid number of arguments.
export function expressionComplete(node: ExpressionNode): boolean {
	switch (node.kind) {
		case "field":
			return node.field !== "";
		case "value":
			return typeof node.value === "number" && Number.isFinite(node.value);
		case "func": {
			if (node.fn === coinFunction) {
				const [, operand, ...rest] = node.args;
				return (
					coinOf(node) !== "" &&
					operand !== undefined &&
					rest.length === 0 &&
					expressionComplete(operand)
				);
			}
			const definition = strategyFunctions[node.fn];
			if (!definition) return false;
			const [fewest, most] = argumentRange(definition);
			const count = node.args.length;
			return (
				count >= fewest && count <= most && node.args.every(expressionComplete)
			);
		}
		default:
			return false;
	}
}

// The first field an expression reads, which names the rule's field.
export function firstField(node: ExpressionNode): string | undefined {
	if (node.kind === "field") return node.field;
	if (node.kind === "func") {
		for (const arg of node.args) {
			const field = firstField(arg);
			if (field) return field;
		}
	}
	return undefined;
}
