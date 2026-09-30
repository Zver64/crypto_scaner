import {
	defaultRuleProcessorCEL,
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
import type { StrategyVariable } from "@/api/generated/models";
import { chartIntervalOptions } from "@/components/price-history-chart/config";
import type {
	StrategyOperator,
	StrategyQuery,
} from "@/features/strategy-settings/types";

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
// other operators one number or indicator.
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
	return valueSource === "field" ? value : numberOrZero(value);
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

function crossProcessor(
	rule: Parameters<RuleProcessor>[0],
	options: Parameters<RuleProcessor>[1],
): string {
	const name = crossFunction(rule.operator);
	return name
		? `${name}(${rule.field}, ${String(rule.value)})`
		: defaultRuleProcessorCEL(rule, options);
}

// The CEL expression the backend evaluates.
export function strategyExpression(query: StrategyQuery): string {
	return formatQuery(query, { format: "cel", ruleProcessor: crossProcessor });
}

// Restores the builder query from a stored expression. Unknown parts are
// dropped, so an expression the builder cannot show opens incomplete.
export function strategyQuery(expression: string): StrategyQuery {
	const parsed = parseCEL(expression, {
		customExpressionHandler: crossRule,
	});
	return normalizeGroup(parsed) as StrategyQuery;
}

function crossRule(expression: CELExpression): RuleType | null {
	if (expression.type !== "FunctionCall") return null;
	const call = expression as CELFunctionCall;
	const operator = crossOperator(call.name.value);
	const [left, right] = call.args.value;
	if (!operator || left?.type !== "Identifier" || !right) return null;
	const field = (left as CELExpression & { value: string }).value;
	const value = (right as CELExpression & { value: unknown }).value;
	switch (right.type) {
		case "Identifier":
			return { field, operator, value, valueSource: "field" };
		case "IntegerLiteral":
		case "FloatLiteral":
			return { field, operator, value };
		default:
			return null;
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
	return operator
		? { field: low.field, operator, value: [low.value, high.value] }
		: group;
}

// Whether every rule names an indicator and has numeric values, and the query
// has at least one rule.
export function strategyQueryComplete(query: RuleGroupType): boolean {
	return (
		query.rules.length > 0 &&
		query.rules.every((rule) =>
			isRuleGroup(rule) ? strategyQueryComplete(rule) : ruleComplete(rule),
		)
	);
}

function ruleComplete(rule: RuleType): boolean {
	if (!rule.field) return false;
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

// Validation and conflict messages come from the backend.
export function strategyErrorMessage(error: unknown): string {
	const info = (
		error as { info?: { error?: { code?: unknown; message?: unknown } } }
	).info?.error;
	switch (info?.code) {
		case "invalid_argument":
			return typeof info.message === "string"
				? info.message
				: "The strategy could not be saved.";
		case "strategy_exists":
			return "Another strategy has this name.";
		case "strategy_not_found":
			return "This strategy no longer exists.";
		case "administrator_required":
		case "access_denied":
			return "Only the scanner administrator can change strategies.";
		case "unauthenticated":
			return "Telegram authorization has expired. Reopen the Mini App.";
		default:
			return "The strategy could not be saved.";
	}
}

// Select data for indicator variables, grouped by candle interval.
export function variableSelectData(variables: readonly StrategyVariable[]) {
	const groups = new Map<string, { label: string; value: string }[]>();
	for (const variable of variables) {
		const group =
			chartIntervalOptions.find(({ value }) => value === variable.interval)
				?.label ?? variable.interval;
		const items = groups.get(group) ?? [];
		items.push({ label: variable.label, value: variable.name });
		groups.set(group, items);
	}
	return [...groups].map(([group, items]) => ({ group, items }));
}
