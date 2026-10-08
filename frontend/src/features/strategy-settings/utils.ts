import type { ExpressionNode } from "@react-querybuilder/expr";
import {
	formatQuery,
	isRuleGroup,
	type RuleGroupType,
	type RuleProcessor,
	type RuleType,
} from "react-querybuilder";
import {
	type CELExpression,
	type CELFunctionCall,
	parseCEL,
} from "react-querybuilder/parseCEL";
import type { ErrorType } from "@/api/fetch";
import type {
	ErrorResponse,
	Strategy,
	StrategyVariable,
} from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import {
	anyField,
	arithmeticTokens,
	comparisonTokens,
	operandBoundaries,
	tokenPattern,
} from "@/features/strategy-settings/constants";
import {
	callSource,
	expressionComplete,
	expressionParser,
	expressionRuleProcessor,
	expressionSource,
	firstField,
} from "@/features/strategy-settings/expressions";
import type {
	StrategyDraft,
	StrategyOperator,
	StrategyQuery,
	Token,
} from "@/features/strategy-settings/types";
import { describeApiError } from "@/utils/api-error";
import { formatMarketCapUsd, usdPerMillion } from "@/utils/market-cap";

export function isRangeOperator(operator: string): boolean {
	return operator === "between" || operator === "notBetween";
}

export function isCrossOperator(operator: string): boolean {
	return operator === "crossesAbove" || operator === "crossesBelow";
}

export function emptyStrategyQuery(): StrategyQuery {
	return { combinator: "and", rules: [] };
}

// The value a rule takes after its operator changes: ranges take two numbers,
// other operators one number, indicator, or expression.
export function valueForOperator(
	operator: string,
	value: unknown,
	valueSource: string | undefined,
): unknown {
	if (isRangeOperator(operator)) {
		return Array.isArray(value)
			? value
			: [numberOrZero(value), numberOrZero(value)];
	}
	if (Array.isArray(value)) return numberOrZero(value[0]);
	return valueSource === "field" || valueSource === "expression"
		? value
		: numberOrZero(value);
}

function numberOrZero(value: unknown): number {
	return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

// The CEL function of a crossing operator.
function crossFunction(operator: string): string | undefined {
	switch (operator) {
		case "crossesAbove":
			return "crosses_above";
		case "crossesBelow":
			return "crosses_below";
		default:
			return undefined;
	}
}

// The crossing operator of a CEL function.
function crossOperator(name: string): StrategyOperator | undefined {
	switch (name) {
		case "crosses_above":
			return "crossesAbove";
		case "crosses_below":
			return "crossesBelow";
		default:
			return undefined;
	}
}

// Crossings take any operands, and ranges of an expression are written like
// the stock ranges of a field; other rules go to the expression processor,
// which exports plain rules as before.
function strategyRuleProcessor(
	rule: Parameters<RuleProcessor>[0],
	options: Parameters<RuleProcessor>[1],
): string {
	if (rule.lhs && isRangeOperator(rule.operator)) {
		return expressionRange(rule.lhs, rule.operator, rule.value);
	}
	const name = crossFunction(rule.operator);
	if (!name) return expressionRuleProcessor(rule, options);
	const left = rule.lhs ? expressionSource(rule.lhs) : rule.field;
	const right =
		rule.valueSource === "expression"
			? expressionSource(rule.value as ExpressionNode)
			: String(rule.value);
	return callSource(name, [left, right]);
}

// A range of an expression with its bounds in ascending order.
function expressionRange(
	lhs: ExpressionNode,
	operator: string,
	value: unknown,
): string {
	const bounds = Array.isArray(value) ? value.map(Number) : [];
	const [low, high] = [Math.min(...bounds), Math.max(...bounds)];
	const left = expressionSource(lhs);
	return operator === "between"
		? `(${left} >= ${low} && ${left} <= ${high})`
		: `(${left} < ${low} || ${left} > ${high})`;
}

// The CEL expression the backend evaluates.
export function strategyExpression(query: StrategyQuery): string {
	return formatQuery(query, {
		format: "cel",
		ruleProcessor: strategyRuleProcessor,
	});
}

// Restores the builder query from a stored expression, which may be
// written by hand without parentheses around arithmetic operands. Unknown
// parts are dropped, so an expression the builder cannot show opens
// incomplete.
export function strategyQuery(expression: string): StrategyQuery {
	const parsed = parseCEL(parenthesizeOperands(expression), {
		customExpressionHandler: crossRule,
		getExpression: expressionParser,
	});
	return normalizeGroup(parsed) as StrategyQuery;
}

// The builder query of an imported expression, or undefined when the
// builder cannot show all of it, which importing would silently drop.
export function importedStrategyQuery(
	expression: string,
): StrategyQuery | undefined {
	const query = strategyQuery(expression);
	return strategyQueryComplete(query) &&
		!strategyQueryDropped(expression, query)
		? query
		: undefined;
}

// The parser restores arithmetic operands of comparisons only in
// parentheses, as the builder writes them, so h_volume >= 1.5 * h_sma becomes
// h_volume >= (1.5 * h_sma). A negative number stays bare.
export function parenthesizeOperands(expression: string): string {
	const tokens: Token[] = [];
	for (const match of expression.matchAll(tokenPattern)) {
		if (match[0].trim() === "") continue;
		tokens.push({
			text: match[0],
			start: match.index,
			end: match.index + match[0].length,
		});
	}
	const wraps: [number, number][] = [];
	tokens.forEach((token, index) => {
		if (!comparisonTokens.includes(token.text)) return;
		for (const [first, last] of [
			[operandStart(tokens, index), index - 1],
			[index + 1, operandEnd(tokens, index)],
		] as const) {
			const operand = tokens.slice(first, last + 1);
			if (needsParentheses(operand)) {
				wraps.push([tokens[first].start, tokens[last].end]);
			}
		}
	});
	let result = expression;
	for (const [start, end] of wraps.sort(([a], [b]) => b - a)) {
		result = `${result.slice(0, start)}(${result.slice(start, end)})${result.slice(end)}`;
	}
	return result;
}

function operandStart(tokens: readonly Token[], comparison: number): number {
	let depth = 0;
	for (let index = comparison - 1; index >= 0; index--) {
		const { text } = tokens[index];
		if (text === ")") depth++;
		else if (text === "(") {
			if (depth === 0) return index + 1;
			depth--;
		} else if (
			depth === 0 &&
			(operandBoundaries.includes(text) || text === "!")
		)
			return index + 1;
	}
	return 0;
}

function operandEnd(tokens: readonly Token[], comparison: number): number {
	let depth = 0;
	for (let index = comparison + 1; index < tokens.length; index++) {
		const { text } = tokens[index];
		if (text === "(") depth++;
		else if (text === ")") {
			if (depth === 0) return index - 1;
			depth--;
		} else if (depth === 0 && operandBoundaries.includes(text))
			return index - 1;
	}
	return tokens.length - 1;
}

// Whether an operand calculates outside parentheses.
function needsParentheses(operand: readonly Token[]): boolean {
	const [sign, number] = operand;
	if (operand.length === 2 && sign?.text === "-" && /^[\d.]/.test(number.text))
		return false;
	let depth = 0;
	return operand.some(({ text }) => {
		if (text === "(") depth++;
		else if (text === ")") depth--;
		return depth === 0 && arithmeticTokens.includes(text);
	});
}

// Whether restoring expression dropped conditions the builder cannot show.
// The builder may rewrite kept conditions, such as dropping outer parentheses
// or swapping the sides of a comparison, so only the comparisons are counted.
export function strategyQueryDropped(
	expression: string,
	query: StrategyQuery,
): boolean {
	return (
		comparisonCount(strategyExpression(query)) < comparisonCount(expression)
	);
}

function comparisonCount(expression: string): number {
	return expression.match(/crosses_(?:above|below)\(|[<>]=?/g)?.length ?? 0;
}

function crossRule(expression: CELExpression): RuleType | null {
	if (expression.type !== "FunctionCall") return null;
	const call = expression as CELFunctionCall;
	const operator = crossOperator(call.name.value);
	const [left, right] = call.args.value;
	if (!operator || !left || !right) return null;
	let field: string;
	let lhs: ExpressionNode | undefined;
	if (left.type === "Identifier") {
		field = (left as CELExpression & { value: string }).value;
	} else {
		lhs = expressionParser(left, anyField) ?? undefined;
		field = lhs ? (firstField(lhs) ?? "") : "";
		if (!lhs) return null;
	}
	const value = (right as CELExpression & { value: unknown }).value;
	const rule: RuleType = { field, operator, value, ...(lhs ? { lhs } : {}) };
	switch (right.type) {
		case "Identifier":
			return { ...rule, valueSource: "field" };
		case "IntegerLiteral":
		case "FloatLiteral":
			return rule;
		default: {
			const node = expressionParser(right, anyField);
			return node ? { ...rule, value: node, valueSource: "expression" } : null;
		}
	}
}

// parseCEL spells ranges as two comparisons and wraps a negated group in
// another group; turn them back into what the builder produced.
function normalizeGroup(group: RuleGroupType): RuleGroupType {
	const rules = group.rules.map((rule) =>
		isRuleGroup(rule) ? rangeRule(normalizeGroup(rule)) : rule,
	);
	const [only] = rules;
	if (group.not && rules.length === 1 && only && isRuleGroup(only)) {
		return { ...only, not: !only.not };
	}
	return { ...group, rules };
}

function rangeRule(group: RuleGroupType): RuleGroupType | RuleType {
	const [low, high] = group.rules;
	if (
		group.not ||
		group.rules.length !== 2 ||
		!low ||
		!high ||
		isRuleGroup(low) ||
		isRuleGroup(high) ||
		low.field !== high.field ||
		// Comparisons of expressions are a range only of the same expression.
		JSON.stringify(low.lhs) !== JSON.stringify(high.lhs) ||
		low.valueSource === "field" ||
		high.valueSource === "field" ||
		typeof low.value !== "number" ||
		typeof high.value !== "number" ||
		// The exporter orders range bounds, so only an ordered pair keeps
		// its meaning as a range.
		low.value > high.value
	) {
		return group;
	}
	const pair = `${group.combinator} ${low.operator} ${high.operator}`;
	const operator: StrategyOperator | undefined =
		pair === "and >= <="
			? "between"
			: pair === "or < >"
				? "notBetween"
				: undefined;
	if (!operator) return group;
	const range = { field: low.field, operator, value: [low.value, high.value] };
	return low.lhs ? { ...range, lhs: low.lhs } : range;
}

// Whether every rule names an indicator or a complete expression and has
// numeric values, and the query has at least one rule.
export function strategyQueryComplete(query: RuleGroupType): boolean {
	return (
		query.rules.length > 0 &&
		query.rules.every((rule) =>
			isRuleGroup(rule) ? strategyQueryComplete(rule) : ruleComplete(rule),
		)
	);
}

function ruleComplete(rule: RuleType): boolean {
	if (rule.lhs ? !expressionComplete(rule.lhs) : !rule.field) return false;
	if (rule.valueSource === "expression") {
		return expressionComplete(rule.value as ExpressionNode);
	}
	if (rule.valueSource === "field") {
		return typeof rule.value === "string" && rule.value !== "";
	}
	const values: unknown[] = Array.isArray(rule.value)
		? rule.value
		: [rule.value];
	return (
		values.length === (isRangeOperator(rule.operator) ? 2 : 1) &&
		values.every((value) => typeof value === "number" && Number.isFinite(value))
	);
}

// The draft of a new strategy.
export function newStrategyDraft(): Omit<StrategyDraft, "revision"> {
	return {
		id: undefined,
		name: "",
		message: "",
		query: emptyStrategyQuery(),
		exitQuery: undefined,
		takeProfit: "",
		stopLoss: "",
		minMarketCap: "",
		maxMarketCap: "",
		incomplete: false,
	};
}

// The draft editing strategy, with its rules parsed into the builder.
export function strategyDraft(
	strategy: Strategy,
): Omit<StrategyDraft, "revision"> {
	const query = strategyQuery(strategy.expression);
	const exitQuery =
		strategy.exit_expression === ""
			? undefined
			: strategyQuery(strategy.exit_expression);
	return {
		id: strategy.id,
		name: strategy.name,
		message: strategy.message,
		query,
		exitQuery,
		takeProfit: strategy.take_profit_expression,
		stopLoss: strategy.stop_loss_expression,
		minMarketCap: marketCapMillions(strategy.min_market_cap_usd),
		maxMarketCap: marketCapMillions(strategy.max_market_cap_usd),
		incomplete:
			strategyQueryDropped(strategy.expression, query) ||
			(exitQuery !== undefined &&
				strategyQueryDropped(strategy.exit_expression, exitQuery)),
	};
}

// A market cap bound in millions of USD for the form; empty without one.
function marketCapMillions(usd: number | null): number | "" {
	return usd === null ? "" : usd / usdPerMillion;
}

// The market cap bound in USD of a form value in millions, which is a
// string while being typed; null, an open bound, when empty, and NaN when
// not a number.
export function marketCapUsd(millions: number | string): number | null {
	if (millions === "") return null;
	return Math.round(Number(millions) * usdPerMillion);
}

// Why the market cap bounds in USD cannot be saved, for the minimum and the
// maximum field; undefined for a valid one.
export function marketCapErrors(
	minimum: number | null,
	maximum: number | null,
): { maximum?: string; minimum?: string } {
	const positive = (usd: number | null) =>
		usd === null || usd > 0 ? undefined : "Must be a positive amount";
	const errors = { maximum: positive(maximum), minimum: positive(minimum) };
	if (
		!errors.minimum &&
		!errors.maximum &&
		minimum !== null &&
		maximum !== null &&
		minimum > maximum
	) {
		errors.maximum = "Must not be below the minimum";
	}
	return errors;
}

// The market cap range of a strategy, such as "$10M – $1.5B"; undefined
// without bounds.
export function marketCapRangeLabel(
	strategy: Pick<Strategy, "max_market_cap_usd" | "min_market_cap_usd">,
): string | undefined {
	const minimum = strategy.min_market_cap_usd;
	const maximum = strategy.max_market_cap_usd;
	if (minimum !== null && maximum !== null) {
		return `${formatMarketCapUsd(minimum)} – ${formatMarketCapUsd(maximum)}`;
	}
	if (minimum !== null) return `≥ ${formatMarketCapUsd(minimum)}`;
	if (maximum !== null) return `≤ ${formatMarketCapUsd(maximum)}`;
	return undefined;
}

// Whether a strategy sells, through an exit rule, take profit, or stop loss;
// such a strategy holds one buy per trade.
export function strategyExits(
	strategy: Pick<
		Strategy,
		"exit_expression" | "stop_loss_expression" | "take_profit_expression"
	>,
): boolean {
	return (
		strategy.exit_expression !== "" ||
		strategy.take_profit_expression !== "" ||
		strategy.stop_loss_expression !== ""
	);
}

// How a strategy buys: once per trade when it exits, or at every entry
// signal without any exit.
export function strategyBuysLabel(
	strategy: Parameters<typeof strategyExits>[0],
): string {
	return strategyExits(strategy)
		? "One buy per trade"
		: "Buys at every entry signal and never sells";
}

// The title of the form editing draft.
export function strategyFormTitle(draft: StrategyDraft): string {
	return draft.id === undefined ? "New strategy" : "Edit strategy";
}

// Validation and conflict messages come from the backend.
export function strategyErrorMessage(
	error: ErrorType<ErrorResponse> | null,
): string {
	return describeApiError(error, {
		fallback: "The strategy could not be saved.",
		forbidden: "Only the scanner administrator can change strategies.",
		messages: {
			strategy_exists: "Another strategy has this name.",
			strategy_not_found: "This strategy no longer exists.",
		},
		server: ["invalid_argument"],
	});
}

// The variables that exist at every candle and of every coin: all but the
// position variables of the open trade, which the backend reads only at the
// latest candle of the evaluated coin.
export function withoutPositionVariables(
	variables: readonly StrategyVariable[],
): StrategyVariable[] {
	return variables.filter(({ position }) => !position);
}

// Select data for variables, grouped by candle interval; the position
// variables of exit rules form their own group.
export function variableSelectData(variables: readonly StrategyVariable[]) {
	const groups = new Map<string, { label: string; value: string }[]>();
	for (const variable of variables) {
		const group =
			variable.interval === undefined
				? "Position"
				: (chartIntervalOptions.find(({ value }) => value === variable.interval)
						?.label ?? variable.interval);
		const items = groups.get(group) ?? [];
		items.push({ label: variable.label, value: variable.name });
		groups.set(group, items);
	}
	return [...groups].map(([group, items]) => ({ group, items }));
}
