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
import type {
	CELExpression,
	CELFunctionCall,
	CELIdentifier,
	CELNumericLiteral,
	CELStringLiteral,
} from "react-querybuilder/parseCEL";

import type { StrategyVariable } from "@/features/strategy-settings/types";

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
	// Reads its first argument, the value, at earlier candles, where
	// position variables do not exist; the others at the latest candle.
	earlier?: true;
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
	Min: {
		label: "Min",
		args: ["Value", "Value"],
		defaults: [number(0)],
		variadic: true,
	},
	Max: {
		label: "Max",
		args: ["Value", "Value"],
		defaults: [number(0)],
		variadic: true,
	},
	Abs: { label: "Abs", args: ["Value"], defaults: [] },
	Mod: { label: "Mod", args: ["Value", "Divisor"], defaults: [number(1)] },
	Prev: {
		label: "Prev — earlier candle",
		args: ["Value", "Candles back"],
		defaults: [number(1)],
		// Prev(x) reads one candle back.
		minArgs: 1,
		earlier: true,
	},
	Percentile: {
		label: "Percentile — of earlier candles",
		args: ["Value", "Candles", "Percentile"],
		defaults: [number(120), number(20)],
		earlier: true,
	},
	ValueWhen: {
		label: "ValueWhen — earlier value",
		args: ["Value", "Values back"],
		defaults: [number(0)],
		// ValueWhen(x) reads the latest value.
		minArgs: 1,
		earlier: true,
	},
	BarsSince: {
		label: "BarsSince — candles since a value",
		args: ["Value"],
		defaults: [],
		earlier: true,
	},
};

// Of("BTCUSDT", x) reads the operand x of another coin. It is not a
// function over values, so the builder offers it as an operand kind of its
// own rather than among strategyFunctions.
const coinFunction = "Of";

// Min and Max take up to ten values on the backend.
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
	"Min",
	"Max",
	"Abs",
	"Mod",
	"Prev",
	"Percentile",
	"ValueWhen",
	"BarsSince",
	coinFunction,
];

const serializers: SQLSerializerRegistry = {
	...defaultCELSerializers,
	...Object.fromEntries(
		callNames.map((name) => [
			name,
			(_options: unknown, ...args: string[]) => callSource(name, args),
		]),
	),
};

// A call of name. Arguments are separated by commas, so the parentheses the
// serializer puts around arithmetic are left out.
export function callSource(name: string, args: readonly string[]): string {
	return `${name}(${args.map(withoutOuterParentheses).join(", ")})`;
}

// source without the parentheses enclosing all of it, such as (a - b) but
// not (a - b) / (c - d).
function withoutOuterParentheses(source: string): string {
	if (!source.startsWith("(")) return source;
	let depth = 0;
	let quote: string | undefined;
	for (let index = 0; index < source.length; index++) {
		const character = source[index];
		if (quote) {
			if (character === "\\") index++;
			else if (character === quote) quote = undefined;
		} else if (character === '"' || character === "'") quote = character;
		else if (character === "(") depth++;
		else if (character === ")") {
			depth--;
			if (depth === 0) {
				return index === source.length - 1
					? withoutOuterParentheses(source.slice(1, -1))
					: source;
			}
		}
	}
	return source;
}

// Exports rules whose sides hold expressions.
export const expressionRuleProcessor =
	getExpressionRuleProcessorCEL(serializers);

const functionParser = getExpressionParserCEL(
	{ functions: Object.fromEntries(callNames.map((name) => [name, name])) },
	functionMeta,
);

// Reads arithmetic and function operands of stored expressions. Indicator
// and candle field calls are variables, read as fields named by their call.
export const expressionParser: typeof functionParser = (expression, context) =>
	functionParser(withVariableCalls(expression), context);

// expression with every indicator and candle field call it holds replaced
// by a field named by the call.
function withVariableCalls(expression: CELExpression): CELExpression {
	const source = variableCallSource(expression);
	if (source !== undefined) {
		const field: CELIdentifier = { type: "Identifier", value: source };
		return field;
	}
	return Object.fromEntries(
		Object.entries(expression).map(([key, value]) => [
			key,
			withVariableCallsIn(value),
		]),
	) as unknown as CELExpression;
}

function withVariableCallsIn(value: unknown): unknown {
	if (Array.isArray(value)) return value.map(withVariableCallsIn);
	return typeof value === "object" && value !== null && "type" in value
		? withVariableCalls(value as CELExpression)
		: value;
}

// The source of an indicator or candle field call, such as
// sma("1h", 20, volume): a lowercase name, a period, and literal
// parameters. Undefined for anything else, such as a function call, whose
// name is capitalized.
function variableCallSource(expression: CELExpression): string | undefined {
	if (expression.type !== "FunctionCall") return undefined;
	const call = expression as CELFunctionCall;
	const name = call.name.value;
	const [period, ...parameters] = call.args.value;
	if (!/^[a-z]/.test(name) || period?.type !== "StringLiteral") {
		return undefined;
	}
	const args = [
		JSON.stringify((period as CELStringLiteral).value.slice(1, -1)),
	];
	for (const parameter of parameters) {
		if (
			parameter.type === "IntegerLiteral" ||
			parameter.type === "FloatLiteral"
		) {
			args.push(canonicalNumber((parameter as CELNumericLiteral).value));
		} else if (parameter.type === "Identifier") {
			args.push((parameter as CELIdentifier).value);
		} else {
			return undefined;
		}
	}
	return `${name}(${args.join(", ")})`;
}

// value spelled as the backend spells canonical numbers: the shortest
// decimal that reads back as value, without an exponent, such as 0.0000001
// for 1e-7. The parser keeps only the value of a literal, not its source.
function canonicalNumber(value: number): string {
	const [mantissa, exponent] = String(value).split("e");
	if (exponent === undefined) return mantissa;
	const [whole, fraction = ""] = mantissa.split(".");
	const digits = whole + fraction;
	const point = whole.length + Number(exponent);
	if (point <= 0) return `0.${"0".repeat(-point)}${digits}`;
	if (point >= digits.length) return digits + "0".repeat(point - digits.length);
	return `${digits.slice(0, point)}.${digits.slice(point)}`;
}

// The call reading variable on interval, such as bbands_upperband("1h", 20);
// a position variable is read by its name.
export function variableCall(
	variable: StrategyVariable,
	interval: string,
): string {
	if (variable.kind === "position") return variable.name;
	const args = [JSON.stringify(interval), ...variable.parameters].join(", ");
	return `${variable.name}(${args})`;
}

// The key of a variable: its call without the period, such as
// bbands_upperband(20), or the name of a position variable.
export function variableKey(
	variable: Pick<StrategyVariable, "kind" | "name" | "parameters">,
): string {
	if (variable.kind === "position") return variable.name;
	return `${variable.name}(${variable.parameters.join(", ")})`;
}

// field read on interval instead of its own period.
export function withInterval(field: string, interval: string): string {
	return field.replace(/^(\w+\()"[^"]*"/, `$1${JSON.stringify(interval)}`);
}

// What the field of an operand reads: the key of its variable and, for a
// call, its period.
export function fieldVariable(field: string): {
	key: string;
	interval?: string;
} {
	const match = /^(\w+)\("([^"]*)"(?:, )?/.exec(field);
	if (!match) return { key: field };
	const [prefix, name, interval] = match;
	return { key: `${name}(${field.slice(prefix.length)}`, interval };
}

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
