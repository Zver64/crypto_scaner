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

import {
	type IndicatorChoice,
	type IndicatorParameter,
	IndicatorParameterKind,
} from "@/api/generated/models";
import { indicatorSourceItem } from "@/features/strategy-settings/constants";
import type {
	IndicatorArgument,
	IndicatorCallParts,
	StrategyIndicatorVariable,
	StrategyVariable,
} from "@/features/strategy-settings/types";

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
	// Starts with its first argument repeated as the second one.
	repeatsFirst?: true;
	// The number of leading arguments it reads at earlier candles, where
	// position variables do not exist; the others at the latest candle.
	earlier?: number;
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
		earlier: 1,
	},
	Percentile: {
		label: "Percentile — of earlier candles",
		args: ["Value", "Candles", "Percentile"],
		defaults: [number(120), number(20)],
		earlier: 1,
	},
	ValueWhen: {
		label: "ValueWhen — earlier value",
		args: ["When", "Value", "Values back"],
		defaults: [number(0)],
		// ValueWhen(x, x, 0) is the latest value of x.
		repeatsFirst: true,
		earlier: 2,
	},
	BarsSince: {
		label: "BarsSince — candles since a value",
		args: ["Value"],
		defaults: [],
		earlier: 1,
	},
};

// Of("BTCUSDT", x) reads the operand x of another coin. It is not a
// function over values, so the builder offers it as an operand kind of its
// own rather than among strategyFunctions.
export const coinFunction = "Of";

// Min and Max take up to ten values on the backend.
export const maxVariadicArguments = 10;

// The inclusive range of argument counts a function accepts.
export function argumentRange(definition: StrategyFunction): [number, number] {
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
// parameters, of which one may be a bare indicator call without one of its
// own, such as pivot_low("1h", 5, 5, rsi("1h", 21)). Undefined for anything
// else, such as a function call, whose name is capitalized.
function variableCallSource(
	expression: CELExpression,
	nested = false,
): string | undefined {
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
			const source = nested ? undefined : variableCallSource(parameter, true);
			if (source === undefined) return undefined;
			args.push(source);
		}
	}
	return `${name}(${args.join(", ")})`;
}

// value spelled as the backend spells canonical numbers: the shortest
// decimal that reads back as value, without an exponent, such as 0.0000001
// for 1e-7. The parser keeps only the value of a literal, not its source.
export function canonicalNumber(value: number): string {
	const [mantissa, exponent] = String(value).split("e");
	if (exponent === undefined) return mantissa;
	const [whole, fraction = ""] = mantissa.split(".");
	const digits = whole + fraction;
	const point = whole.length + Number(exponent);
	if (point <= 0) return `0.${"0".repeat(-point)}${digits}`;
	if (point >= digits.length) return digits + "0".repeat(point - digits.length);
	return `${digits.slice(0, point)}.${digits.slice(point)}`;
}

// The call reading variable on interval with the default parameters, such
// as bbands_upperband("1h"); a position variable is read by its name.
export function variableCall(
	variable: StrategyVariable,
	interval: string,
): string {
	if (variable.kind === "position") return variable.name;
	return `${variable.name}(${JSON.stringify(interval)})`;
}

// field read on interval instead of its own period, with the indicator it
// reads as its source, which reads the same period.
export function withInterval(field: string, interval: string): string {
	return field.replaceAll(/"[^"]*"/g, JSON.stringify(interval));
}

// What the field of an operand reads: the name of its variable and, for a
// call, its period.
export function fieldVariable(field: string): {
	name: string;
	interval?: string;
} {
	const match = /^(\w+)\("([^"]*)"/.exec(field);
	if (!match) return { name: field };
	const [, name, interval] = match;
	return { name, interval };
}

// A call as written: its name, its period, and the arguments after the
// period as their source, split at the commas outside the parentheses of a
// call written as an argument, such as ["5", "5", 'rsi("1h", 21)'].
function callParts(
	source: string,
): { name: string; interval: string; args: string[] } | undefined {
	const match = /^(\w+)\("([^"]*)"(.*)\)$/s.exec(source);
	if (!match) return undefined;
	const [, name, interval, rest] = match;
	if (rest === "") return { name, interval, args: [] };
	if (!rest.startsWith(",")) return undefined;
	const args: string[] = [];
	let depth = 0;
	let start = 1;
	for (let index = 1; index <= rest.length; index++) {
		const character = rest[index];
		if (character === "(") depth++;
		else if (character === ")") depth--;
		else if (index === rest.length || (character === "," && depth === 0)) {
			args.push(rest.slice(start, index).trim());
			start = index + 1;
		}
	}
	return depth === 0 ? { name, interval, args } : undefined;
}

// A number a call writes as a parameter.
const numberArgument = /^-?(?:\d+|\d*\.\d+)$/;

// arg spelled canonically when it is a number, such as 2 for 2.0 or 14.5
// for 14.50; any other argument as it is.
export function canonicalArgument(arg: string): string {
	return numberArgument.test(arg) ? canonicalNumber(Number(arg)) : arg;
}

// A name a call writes as a source, such as volume.
const nameArgument = /^[a-z_]\w*$/;

// The positions of the arguments of an indicator call that are not filled
// in: neither a number, a name, nor a call; those of a call written as a
// source follow the position of that call, such as [3, 0]. None for a field
// that is not a call with arguments.
export function callArgumentIssues(field: string): number[][] {
	const call = callParts(field);
	if (!call) return [];
	const filled = (arg: string) =>
		numberArgument.test(arg) || nameArgument.test(arg);
	return call.args.flatMap((arg, index) => {
		if (filled(arg)) return [];
		const nested = callParts(arg);
		if (!nested) return [[index]];
		return nested.args.flatMap((inner, position) =>
			filled(inner) ? [] : [[index, position]],
		);
	});
}

// The argument a call writes for choice of parameter: a named choice by its
// name, such as volume or price, a source choice without one empty, as only
// leaving the source out writes it, and any other choice by its value.
export function choiceArgument(
	parameter: IndicatorParameter,
	choice: IndicatorChoice,
): string {
	if (choice.name !== undefined) return choice.name;
	return parameter.kind === IndicatorParameterKind.source
		? ""
		: canonicalNumber(choice.value);
}

// The argument a call leaves out for parameter: its default.
export function defaultArgument(parameter: IndicatorParameter): string {
	const choice = parameter.choices?.find(
		({ value }) => value === parameter.default,
	);
	return choice
		? choiceArgument(parameter, choice)
		: canonicalNumber(parameter.default);
}

// The indicator call field reads, with every argument; undefined for a
// candle field or position variable, an indicator none of variables reads,
// or a call that does not fit the parameters of its indicator, such as one
// reading a sparse output or a source of a source as its source.
export function indicatorCall(
	field: string,
	variables: readonly StrategyVariable[],
	nested = false,
): IndicatorCallParts | undefined {
	const call = callParts(field);
	const variable = variables.find(
		(item): item is StrategyIndicatorVariable =>
			item.kind === "indicator" && item.name === call?.name,
	);
	if (!call || !variable) return undefined;
	const { parameters } = variable.indicator;
	if (call.args.length > parameters.length) return undefined;
	const args: IndicatorArgument[] = [];
	for (const [index, parameter] of parameters.entries()) {
		const arg = call.args[index] ?? defaultArgument(parameter);
		const source = parameter.kind === IndicatorParameterKind.source && !nested;
		// A source whose indicator is still to be chosen stays as it is.
		if (!arg.includes("(") || (source && arg === indicatorSourceItem.value)) {
			args.push(arg);
			continue;
		}
		const sourceCall = source ? indicatorCall(arg, variables, true) : undefined;
		if (!sourceCall || sourceCall.variable.output.sparse) return undefined;
		args.push(sourceCall);
	}
	return { variable, interval: call.interval, args };
}

// The call of parts as the builder writes it: the parameters in descriptor
// order up to the last one that is not its default, an indicator read as a
// source on the same period.
export function indicatorCallSource({
	variable,
	interval,
	args,
}: IndicatorCallParts): string {
	const { parameters } = variable.indicator;
	const written = args.map((arg) =>
		typeof arg === "string" ? arg : indicatorCallSource({ ...arg, interval }),
	);
	let count = written.length;
	while (
		count > 0 &&
		parameters[count - 1] !== undefined &&
		written[count - 1] === defaultArgument(parameters[count - 1])
	) {
		count--;
	}
	// Joined as they are: callSource would drop the parentheses of a source
	// still to be chosen.
	const all = [JSON.stringify(interval), ...written.slice(0, count)];
	return `${variable.name}(${all.join(", ")})`;
}

// The call of the output variable reads on interval with the default
// parameters, as indicatorCall reads it.
export function defaultIndicatorCall(
	variable: StrategyIndicatorVariable,
	interval: string,
): IndicatorCallParts {
	return {
		variable,
		interval,
		args: variable.indicator.parameters.map(defaultArgument),
	};
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
	const definition = strategyFunctions[name];
	return {
		kind: "func",
		fn: name,
		args: [
			node,
			...(definition?.repeatsFirst ? [node] : []),
			...(definition?.defaults ?? []),
		],
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

// The call of name that replaces node, started from its first argument as
// when name is chosen for it.
export function replaceFunction(
	node: FunctionNode,
	name: string,
): FunctionNode {
	return functionCall(name, node.args[0] ?? number(0));
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
