import type { ExpressionNode } from "@react-querybuilder/expr";
import { isRuleGroup } from "react-querybuilder";
import { parseCEL } from "react-querybuilder/parseCEL";

import {
	expressionParser,
	expressionSource,
} from "@/features/strategy-settings/expressions";
import { priceNodeSchema } from "@/features/strategy-settings/schemas";
import type { PriceSetup } from "@/features/strategy-settings/strategy-price-input/types";

// Parse one numeric operand through the condition builder's CEL parser.
function priceNode(source: string): ExpressionNode | undefined {
	let operand: ExpressionNode | undefined;
	try {
		// The wrapper lets the parser read standalone numbers and indicators too.
		const query = parseCEL(`((${source}) + 0) > 0`, {
			getExpression: (expression, context) => {
				const node = expressionParser(expression, context);
				if (node) operand = node;
				return node;
			},
		});
		const rule = query.rules[0];
		if (
			query.rules.length !== 1 ||
			!rule ||
			isRuleGroup(rule) ||
			rule.operator !== ">" ||
			Number(rule.value) !== 0
		)
			return undefined;
		return operand?.kind === "func" &&
			operand.fn === "add" &&
			operand.args.length === 2 &&
			operand.args[1]?.kind === "value" &&
			operand.args[1].value === 0
			? operand.args[0]
			: undefined;
	} catch {
		return undefined;
	}
}

// The source of an operand, with numbers parsed from their inputs when it is
// filled in; an incomplete one is written as it is, which only tells it apart
// from others.
function priceNodeSource(node: ExpressionNode): string {
	const parsed = priceNodeSchema.safeParse(node);
	return expressionSource(parsed.success ? parsed.data : node);
}

export function parsePriceSetup(source: string): PriceSetup {
	if (!source.trim()) return {};
	const node = priceNode(source);
	return node && priceNodeSchema.safeParse(node).success
		? { node, original: source }
		: { custom: source };
}

// Compare editable operands, not the source-preservation metadata. Incomplete
// operands still differ from a disabled price, even though both export as empty.
export function priceSetupChanged(
	setup: PriceSetup,
	initial: PriceSetup,
): boolean {
	if (setup.custom !== initial.custom) return true;
	if (!setup.node || !initial.node) return setup.node !== initial.node;
	return priceNodeSource(setup.node) !== priceNodeSource(initial.node);
}

export function priceSetupComplete(setup: PriceSetup): boolean {
	if (setup.custom !== undefined) return setup.custom.trim() !== "";
	return !setup.node || priceNodeSchema.safeParse(setup.node).success;
}

export function priceSetupExpression(setup: PriceSetup): string {
	if (setup.custom !== undefined) return setup.custom.trim();
	if (!setup.node) return "";
	const parsed = priceNodeSchema.safeParse(setup.node);
	return parsed.success
		? (setup.original ?? expressionSource(parsed.data))
		: "";
}
