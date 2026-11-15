import type { ExpressionNode } from "@react-querybuilder/expr";
import {
	type Field,
	formatQuery,
	type FullField,
	getFieldData,
	isRuleGroup,
	type Path,
	type RuleGroupType,
	type RuleGroupTypeAny,
	type RuleProcessor,
	type RuleType,
	type Schema,
	update,
	type UpdateValueMap,
	type ValidationMap,
	type ValidationResult,
} from "react-querybuilder";
import {
	type CELExpression,
	type CELFunctionCall,
	parseCEL,
} from "react-querybuilder/parseCEL";
import type { z } from "zod";

import type { ErrorType } from "@/api/fetch";
import {
	Direction,
	type ErrorResponse,
	type Strategy,
	type StrategyCatalog,
	StrategyCoinSource,
	type StrategyIndicatorParameter,
	type StrategyUpdate,
} from "@/api/generated/models";
import { chartIntervalOptions } from "@/features/candle-chart/config";
import {
	anyField,
	arithmeticTokens,
	variableGroups,
	comparisonTokens,
	defaultDirections,
	defaultSignalTargetRatio,
	defaultSignalWindow,
	directionLabels,
	kindNouns,
	operandBoundaries,
	signalTargetRatios,
	signalWindows,
	tokenPattern,
} from "@/features/strategy-settings/constants";
import {
	callSource,
	choiceArgument,
	expressionParser,
	expressionRuleProcessor,
	expressionSource,
	fieldVariable,
	firstField,
	variableCall,
} from "@/features/strategy-settings/expressions";
import {
	strategyQuerySchema,
	strategyRuleSchema,
} from "@/features/strategy-settings/schemas";
import {
	parsePriceSetup,
	priceSetupChanged,
	priceSetupComplete,
	priceSetupExpression,
} from "@/features/strategy-settings/strategy-price-input/utils";
import type {
	CanonicalRules,
	StrategyDraft,
	StrategyFormData,
	StrategyFormValues,
	StrategyIndicatorVariable,
	StrategyKind,
	StrategyOperator,
	StrategyQuery,
	StrategyRuleVariables,
	StrategyVariable,
	StrategyVariables,
	StrategyVariableSets,
	Token,
} from "@/features/strategy-settings/types";
import { describeApiError } from "@/utils/api-error";
import { formatMarketCapUsd, usdPerMillion } from "@/utils/market-cap";
import { shortId } from "@/utils/short-id";

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
// starting from the number compared with, or 0 after an indicator or
// expression; other operators one number, indicator, or expression. Numbers
// stay as their inputs hold them.
export function valueForOperator(
	operator: string,
	value: unknown,
	valueSource: string | undefined,
): unknown {
	if (isRangeOperator(operator)) {
		if (Array.isArray(value)) return value;
		const bound =
			valueSource === "field" || valueSource === "expression" ? 0 : value;
		return [bound, bound];
	}
	return Array.isArray(value) ? value[0] : value;
}

// The rule changes an operator choice makes: a range compares numbers, and
// the value follows the operator.
export function operatorUpdate(
	rule: RuleType,
	operator: string,
): UpdateValueMap {
	const valueSource = isRangeOperator(operator) ? "value" : rule.valueSource;
	return {
		operator,
		valueSource,
		value: valueForOperator(operator, rule.value, valueSource),
	};
}

// The variables a rule's operands offer: a crossing reads the previous candle
// too, which position variables do not have.
export function operandVariables(
	variables: StrategyVariables,
	operator: string,
): StrategyVariables {
	return isCrossOperator(operator)
		? withoutPositionVariables(variables)
		: variables;
}

// The compared side of a rule as an operand: a plain indicator stays the rule
// field, other operands are an expression.
export function leftOperand(rule: RuleType): ExpressionNode {
	return (
		(rule.lhs as ExpressionNode | undefined) ?? {
			kind: "field",
			field: rule.field,
		}
	);
}

// The rule changes an edit of the compared side makes. The field follows the
// expression, so the rule keeps naming a variable; a cleared indicator stays
// empty.
export function leftOperandUpdate(node: ExpressionNode): UpdateValueMap {
	if (node.kind === "field") return { lhs: undefined, field: node.field };
	const field = firstField(node);
	return field ? { lhs: node, field } : { lhs: node };
}

// The side a rule compares with as an operand: a plain indicator or number
// stays the rule value, other operands are an expression.
export function rightOperand(rule: RuleType): ExpressionNode {
	if (rule.valueSource === "expression") return rule.value as ExpressionNode;
	if (rule.valueSource === "field") {
		return { kind: "field", field: String(rule.value ?? "") };
	}
	return { kind: "value", value: rule.value };
}

// The rule changes an edit of the side compared with makes.
export function rightOperandUpdate(node: ExpressionNode): UpdateValueMap {
	if (node.kind === "field") return { valueSource: "field", value: node.field };
	if (node.kind === "value") return { valueSource: "value", value: node.value };
	return { valueSource: "expression", value: node };
}

// Applies changes to the rule at path in one update of the builder's query,
// with the builder's own update options, so onQueryChange sees only the
// finished rule.
export function dispatchRuleUpdate(
	schema: Schema<FullField, string>,
	path: Path,
	changes: UpdateValueMap,
): void {
	schema.dispatchQuery(
		update(schema.getQuery(), changes, path, {
			resetOnFieldChange: schema.resetOnFieldChange,
			resetOnOperatorChange: schema.resetOnOperatorChange,
			getRuleDefaultOperator: schema.getRuleDefaultOperator,
			getRuleDefaultValue: schema.getRuleDefaultValue,
			getValueSources: (field, operator) =>
				schema.getValueSources(field, operator, {
					fieldData: getFieldData(field, schema.fieldMap),
				}),
		}),
	);
}

// The CEL function of a crossing operator.
function crossFunction(operator: string): string | undefined {
	switch (operator) {
		case "crossesAbove":
			return "CrossesAbove";
		case "crossesBelow":
			return "CrossesBelow";
		default:
			return undefined;
	}
}

// The crossing operator of a CEL function.
function crossOperator(name: string): StrategyOperator | undefined {
	switch (name) {
		case "CrossesAbove":
			return "crossesAbove";
		case "CrossesBelow":
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

// A range of an expression with its bounds in ascending order. The expr
// package's CEL processor drops the expression of a range
// (https://github.com/react-querybuilder/react-querybuilder/issues/1137);
// once that is fixed, leave ranges to it and remove this function.
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

// The CEL expression the backend evaluates, with numbers parsed from their
// inputs. An incomplete query is written as it is, which only tells it apart
// from others.
export function strategyExpression(query: StrategyQuery): string {
	const parsed = strategyQuerySchema.safeParse(query);
	return formatQuery(parsed.success ? parsed.data : query, {
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
// parentheses, as the builder writes them, so volume("1h") >= 1.5 * x
// becomes volume("1h") >= (1.5 * x). A negative number stays bare.
// Remove once parseCEL reads such operands itself
// (https://github.com/react-querybuilder/react-querybuilder/issues/1136).
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
	return expression.match(/Crosses(?:Above|Below)\(|[<>]=?/g)?.length ?? 0;
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
		isRuleGroup(rule) ? rangeRule(normalizeGroup(rule)) : plainRule(rule),
	);
	const [only] = rules;
	if (group.not && rules.length === 1 && only && isRuleGroup(only)) {
		return { ...only, not: !only.not };
	}
	return { ...group, rules };
}

// The parser reads a call compared as a whole as an expression; the builder
// keeps it a plain field, as it does for edited conditions.
function plainRule(rule: RuleType): RuleType {
	const { lhs, ...rest } = rule;
	const left = lhs as ExpressionNode | undefined;
	const plain = left?.kind === "field" ? { ...rest, field: left.field } : rule;
	const right = plain.value as ExpressionNode;
	return plain.valueSource === "expression" && right.kind === "field"
		? { ...plain, value: right.field, valueSource: "field" }
		: plain;
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

// Marks every rule and group of the query by id: a rule is valid when
// strategyRuleSchema parses it, else its reasons are the schema's issues; a
// group is valid when it holds at least one rule or group.
export function strategyValidator(query: RuleGroupTypeAny): ValidationMap {
	const validation: ValidationMap = {};
	const walk = (group: RuleGroupTypeAny) => {
		if (group.id) validation[group.id] = group.rules.length > 0;
		for (const rule of group.rules) {
			if (typeof rule === "string") continue;
			if (isRuleGroup(rule)) walk(rule);
			else if (rule.id) {
				const result = strategyRuleSchema.safeParse(rule);
				validation[rule.id] = result.success || {
					valid: false,
					reasons: result.error.issues,
				};
			}
		}
	};
	walk(query);
	return validation;
}

// Whether every rule and group of the query is filled in.
export function strategyQueryComplete(query: RuleGroupType): boolean {
	return strategyQuerySchema.safeParse(query).success;
}

// The paths of the issues that make a rule invalid, relative to the rule.
export function validationIssuePaths(
	validation: boolean | ValidationResult | undefined,
): PropertyKey[][] {
	if (typeof validation !== "object" || !Array.isArray(validation.reasons)) {
		return [];
	}
	const issues: z.core.$ZodIssue[] = validation.reasons;
	return issues.map(({ path }) => path);
}

// The issue paths under key, relative to it.
export function issuePathsUnder(
	paths: readonly PropertyKey[][],
	key: PropertyKey,
): PropertyKey[][] {
	return paths.filter(([first]) => first === key).map((path) => path.slice(1));
}

// The issue paths of an operand's own input, apart from the operands below
// it: those not in an argument, other than the coin of another coin's
// operand.
export function ownIssuePaths(
	paths: readonly PropertyKey[][],
	coin: boolean,
): PropertyKey[][] {
	return paths.filter(
		([first, index]) =>
			first !== "args" || typeof index !== "number" || (coin && index === 0),
	);
}

// The issue paths of an indicator operand relative to the call it reads:
// those of the field of an operand inside a function are under "field".
export function fieldIssuePaths(
	paths: readonly PropertyKey[][],
): PropertyKey[][] {
	return paths.map((path) => (path[0] === "field" ? path.slice(1) : path));
}

// The issue paths of an operand's argument at index, relative to it.
export function argumentIssuePaths(
	paths: readonly PropertyKey[][],
	index: number,
): PropertyKey[][] {
	return issuePathsUnder(issuePathsUnder(paths, "args"), index);
}

// What a number input shows for a number operand's value: the number or the
// text being typed.
export function numberInputValue(value: unknown): number | string {
	return typeof value === "number" || typeof value === "string" ? value : "";
}

// The draft of a new strategy or signal.
export function newStrategyDraft(
	kind: StrategyKind,
): Omit<StrategyDraft, "revision"> {
	return {
		id: undefined,
		signal: kind === "signal",
		incomplete: false,
		values: {
			name: "",
			direction: defaultDirections[kind],
			message: "",
			query: emptyStrategyQuery(),
			exitQuery: undefined,
			takeProfit: {},
			stopLoss: {},
			coins: [],
			coinSource: StrategyCoinSource.CoinSourceSelected,
			minMarketCap: "",
			maxMarketCap: "",
			targetRatio: String(defaultSignalTargetRatio),
			window: String(defaultSignalWindow),
		},
	};
}

// The draft editing strategy, with its canonical rules parsed into the
// builder.
export function strategyDraft(
	strategy: Strategy,
	rules: CanonicalRules,
): Omit<StrategyDraft, "revision"> {
	const query = strategyQuery(rules.expression);
	const exitQuery =
		rules.exit_expression === ""
			? undefined
			: strategyQuery(rules.exit_expression);
	return {
		id: strategy.id,
		signal: strategy.signal,
		incomplete:
			strategyQueryDropped(rules.expression, query) ||
			(exitQuery !== undefined &&
				strategyQueryDropped(rules.exit_expression, exitQuery)),
		values: {
			name: strategy.name,
			direction: strategy.direction,
			message: strategy.message,
			query,
			exitQuery,
			takeProfit: parsePriceSetup(rules.take_profit_expression),
			stopLoss: parsePriceSetup(rules.stop_loss_expression),
			coins: strategy.coins,
			coinSource: strategy.coin_source,
			minMarketCap: marketCapMillions(strategy.min_market_cap_usd),
			maxMarketCap: marketCapMillions(strategy.max_market_cap_usd),
			targetRatio: String(strategy.target_ratio ?? defaultSignalTargetRatio),
			window: String(strategy.window ?? defaultSignalWindow),
		},
	};
}

// The strategy form's values with their trees typed; only the strategy form
// writes them. Drop with StrategyFormData once TanStack Form types recursive
// values.
export function strategyFormValues(data: StrategyFormData): StrategyFormValues {
	return data as StrategyFormValues;
}

// What the form's values send, whether or not they can be saved; the
// footer describes it and the dirty check compares it. A signal trades
// nothing, so it has no exit or price fields and sends them empty; only a
// signal evaluates the favorites or all coins and has a market cap range,
// and only a signal that is not simple has a target ratio and a window,
// which the selects hold as strings.
export function strategyFormUpdate(
	values: StrategyFormValues,
	signal: boolean,
): StrategyUpdate {
	const judged = signal && values.direction !== Direction.simple;
	const coinSource = signal
		? values.coinSource
		: StrategyCoinSource.CoinSourceSelected;
	return {
		coin_source: coinSource,
		coins:
			coinSource === StrategyCoinSource.CoinSourceSelected
				? [...values.coins].sort()
				: [],
		exit_expression:
			!signal && values.exitQuery ? strategyExpression(values.exitQuery) : "",
		expression: strategyExpression(values.query),
		max_market_cap_usd: signal ? marketCapUsd(values.maxMarketCap) : null,
		message: values.message.trim(),
		min_market_cap_usd: signal ? marketCapUsd(values.minMarketCap) : null,
		name: values.name.trim(),
		signal,
		direction: values.direction,
		stop_loss_expression: signal ? "" : priceSetupExpression(values.stopLoss),
		take_profit_expression: signal
			? ""
			: priceSetupExpression(values.takeProfit),
		target_ratio: judged
			? (signalTargetRatios.find(
					(ratio) => String(ratio) === values.targetRatio,
				) ?? null)
			: null,
		window: judged
			? (signalWindows.find((window) => String(window) === values.window) ??
				null)
			: null,
	};
}

// What saving the form sends; undefined while it cannot be saved: without a
// name or coins, with an incomplete rule or price, an invalid market cap
// bound, or a signal that is not simple without its target ratio and
// window.
export function strategyInput(
	values: StrategyFormValues,
	signal: boolean,
): StrategyUpdate | undefined {
	const update = strategyFormUpdate(values, signal);
	const boundErrors = marketCapErrors(
		update.min_market_cap_usd,
		update.max_market_cap_usd,
	);
	if (
		update.name === "" ||
		strategyCoinsError(update.coins, update.coin_source) !== undefined ||
		!strategyFormComplete(values, signal) ||
		boundErrors.minimum ||
		boundErrors.maximum ||
		(signal &&
			update.direction !== Direction.simple &&
			(update.target_ratio === null || update.window === null))
	) {
		return undefined;
	}
	return update;
}

// Whether the rules and prices of the form can be saved: every condition is
// filled in, and a short strategy has both a take profit and a stop loss.
function strategyFormComplete(
	values: StrategyFormValues,
	signal: boolean,
): boolean {
	if (!strategyQueryComplete(values.query)) return false;
	if (signal) return true;
	return (
		(values.exitQuery === undefined ||
			strategyQueryComplete(values.exitQuery)) &&
		priceSetupComplete(values.takeProfit) &&
		priceSetupComplete(values.stopLoss) &&
		(values.direction !== Direction.short ||
			(priceSetupExpression(values.takeProfit) !== "" &&
				priceSetupExpression(values.stopLoss) !== ""))
	);
}

// Whether the form differs from the values it opened with. Rules compare as
// the expressions they save, so builder bookkeeping such as rule ids does
// not count; a signal has no exit or price fields to differ.
export function strategyFormDirty(
	initial: StrategyFormValues,
	values: StrategyFormValues,
	signal: boolean,
): boolean {
	const saved = strategyFormUpdate(initial, signal);
	const input = strategyFormUpdate(values, signal);
	return (
		values.name !== initial.name ||
		values.message !== initial.message ||
		input.direction !== saved.direction ||
		input.expression !== saved.expression ||
		input.coin_source !== saved.coin_source ||
		input.coins.join() !== saved.coins.join() ||
		input.min_market_cap_usd !== saved.min_market_cap_usd ||
		input.max_market_cap_usd !== saved.max_market_cap_usd ||
		input.target_ratio !== saved.target_ratio ||
		input.window !== saved.window ||
		(!signal &&
			(input.exit_expression !== saved.exit_expression ||
				priceSetupChanged(values.takeProfit, initial.takeProfit) ||
				priceSetupChanged(values.stopLoss, initial.stopLoss)))
	);
}

// Why the coins cannot be saved: selected coins need at least one. Undefined
// for valid coins.
export function strategyCoinsError(
	coins: string[],
	coinSource: StrategyCoinSource,
): string | undefined {
	return coinSource !== StrategyCoinSource.CoinSourceSelected ||
		coins.length > 0
		? undefined
		: "Choose at least one coin";
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

// How a strategy buys: never for a signal, one short per trade for a short
// strategy, once per trade when it exits, or at every entry signal without
// any exit.
export function strategyBuysLabel(
	strategy: Parameters<typeof strategyExits>[0] &
		Pick<Strategy, "direction" | "signal">,
): string {
	if (strategy.signal) {
		return `${directionLabels[strategy.direction]} signal, buys nothing`;
	}
	if (strategy.direction === Direction.short) {
		return "One short per trade";
	}
	return strategyExits(strategy)
		? "One buy per trade"
		: "Buys at every entry signal and never sells";
}

// The title of the form editing draft.
export function strategyFormTitle(draft: StrategyDraft): string {
	const noun = kindNouns[strategyKind(draft)].one;
	return draft.id === undefined
		? `New ${noun}`
		: `Edit ${noun} #${shortId(draft.id)}`;
}

// The kind of a strategy or its draft.
export function strategyKind({ signal }: { signal: boolean }): StrategyKind {
	return signal ? "signal" : "strategy";
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
	variables: StrategyVariables,
): StrategyVariables {
	return {
		...variables,
		variables: variables.variables.filter(({ kind }) => kind !== "position"),
	};
}

// Select data for variables: the candle fields, the indicators, and the
// position variables of exit rules, each in a group of its own.
export function variableSelectData(variables: readonly StrategyVariable[]) {
	const groups = new Map<string, { label: string; value: string }[]>();
	for (const variable of variables) {
		const group = variableGroups[variable.kind];
		const items = groups.get(group) ?? [];
		items.push({ label: variable.label, value: variable.name });
		groups.set(group, items);
	}
	return [...groups].map(([group, items]) => ({ group, items }));
}

// The indicator outputs another indicator may read as its source: those
// with a value on every candle.
export function sourceVariables(
	variables: readonly StrategyVariable[],
): StrategyIndicatorVariable[] {
	return variables.filter(
		(variable): variable is StrategyIndicatorVariable =>
			variable.kind === "indicator" && !variable.output.sparse,
	);
}

// Select data for the choices of a choice or source parameter, by the
// argument a call writes for each.
export function choiceSelectData(parameter: StrategyIndicatorParameter) {
	return (parameter.choices ?? []).map((choice) => ({
		label: choice.title,
		value: choiceArgument(parameter, choice),
	}));
}

// Select data for the periods calls read.
export function intervalSelectData(intervals: readonly string[]) {
	return intervals.map((interval) => ({
		label:
			chartIntervalOptions.find(({ value }) => value === interval)?.label ??
			interval,
		value: interval,
	}));
}

// The variables of the entry and the exit rule with their builder fields:
// the candle fields, every output of every indicator type, and the position
// variables.
export function strategyVariableSets(
	catalog: StrategyCatalog,
): StrategyVariableSets {
	const variables: StrategyVariable[] = catalog.candle_fields.map((name) => ({
		kind: "candle",
		label: name,
		name,
	}));
	for (const indicator of catalog.indicator_types) {
		for (const output of indicator.outputs) {
			const several = indicator.outputs.length > 1;
			variables.push({
				kind: "indicator",
				label: several ? `${indicator.type} ${output.name}` : indicator.type,
				name: several ? `${indicator.type}_${output.name}` : indicator.type,
				indicator,
				output,
			});
		}
	}
	for (const { name } of catalog.position_variables) {
		variables.push({ kind: "position", label: name, name });
	}
	const all = { intervals: catalog.intervals, variables };
	return {
		entry: ruleVariables(withoutPositionVariables(all)),
		exit: ruleVariables(all),
	};
}

// The builder's fields: every variable on every period with the default
// parameters, a position variable by its name, as calls grouped like the
// variable select; a new condition compares the default call.
function ruleVariables(variables: StrategyVariables): StrategyRuleVariables {
	const groups = new Map<string, Field[]>();
	for (const variable of variables.variables) {
		const group = variableGroups[variable.kind];
		const calls =
			variable.kind === "position"
				? [variable.name]
				: variables.intervals.map((interval) =>
						variableCall(variable, interval),
					);
		groups.set(group, [
			...(groups.get(group) ?? []),
			...calls.map((call) => ({ label: call, name: call })),
		]);
	}
	return {
		...variables,
		defaultField: defaultVariableCall(variables),
		fields: [...groups].map(([label, options]) => ({ label, options })),
	};
}

// The call of the first variable other than the one field reads, on the
// period of field or else on the first period.
export function defaultVariableCall(
	variables: StrategyVariables,
	field = "",
): string | undefined {
	const interval = fieldVariable(field).interval ?? variables.intervals[0];
	if (interval === undefined) return undefined;
	return variables.variables
		.map((variable) => variableCall(variable, interval))
		.find((call) => call !== field);
}
