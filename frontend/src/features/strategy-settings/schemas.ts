import type { ExpressionNode } from "@react-querybuilder/expr";
import { z } from "zod";

import {
	argumentRange,
	callArgumentIssues,
	coinFunction,
	strategyFunctions,
} from "@/features/strategy-settings/expressions";
import type { StrategyQuery } from "@/features/strategy-settings/types";
import { numberInputSchema } from "@/utils/number-input-schema";

// The id of a saved strategy or signal.
export const strategyIdSchema = z.uuid();

export const strategyNameSchema = z.string().trim().min(1, "Enter a name");

const numberSchema = numberInputSchema("Number");

const functionNames = Object.keys(strategyFunctions);

// The issues of the arguments of the indicator call field that are not
// filled in, at their positions after the period under path.
function argumentIssues(
	field: string,
	path: PropertyKey[] = [],
): z.core.$ZodRawIssue[] {
	return callArgumentIssues(field).map((position) => ({
		code: "custom",
		input: field,
		message: "Enter the parameter",
		path: [...path, ...position],
	}));
}

// A chosen variable, whose call has every argument filled in.
const variableSchema = z
	.string()
	.min(1)
	.check((context) => {
		context.issues.push(...argumentIssues(context.value));
	});

// An operand: a chosen indicator, a number, a known function with a valid
// number of arguments, or, withCoin, an operand of another chosen coin.
// Parses to the same tree with numbers as numbers. Issue paths point at the
// part of the tree that is incomplete.
function operandSchema(withCoin: boolean): z.ZodType<ExpressionNode> {
	const operand: z.ZodType<ExpressionNode> = z.lazy(
		(): z.ZodType<ExpressionNode> => {
			const functionCall = z
				.object({
					kind: z.literal("func"),
					fn: z.enum(functionNames),
					args: z.array(operand),
				})
				.check((context) => {
					const definition = strategyFunctions[context.value.fn];
					if (!definition) return;
					const [fewest, most] = argumentRange(definition);
					const count = context.value.args.length;
					if (count < fewest || count > most) {
						context.issues.push({
							code: "custom",
							input: context.value.args,
							message: "Wrong number of arguments",
							path: ["args"],
						});
					}
				});
			const coinCall = z.object({
				kind: z.literal("func"),
				fn: z.literal(coinFunction),
				args: z.tuple([
					z.object({ kind: z.literal("value"), value: z.string().min(1) }),
					operand,
				]),
			});
			return z.discriminatedUnion("kind", [
				z.object({ kind: z.literal("field"), field: variableSchema }),
				z.object({ kind: z.literal("value"), value: numberSchema }),
				withCoin
					? z.discriminatedUnion("fn", [coinCall, functionCall])
					: functionCall,
			]);
		},
	);
	return operand;
}

// An operand of a condition.
export const expressionNodeSchema = operandSchema(true);

// A take profit or stop loss operand, which reads the evaluated coin only.
export const priceNodeSchema = operandSchema(false);

const comparisonOperatorSchema = z.enum([
	">",
	">=",
	"<",
	"<=",
	"crossesAbove",
	"crossesBelow",
]);

// The compared side of a condition: an expression, or else a chosen field.
const comparedSideSchema = z
	.object({ field: z.string(), lhs: expressionNodeSchema.optional() })
	.check((context) => {
		if (context.value.lhs !== undefined) return;
		if (context.value.field === "") {
			context.issues.push({
				code: "custom",
				input: context.value.field,
				message: "Choose an indicator",
				path: ["field"],
			});
		}
		context.issues.push(...argumentIssues(context.value.field, ["field"]));
	});

// The side a condition compares with: an indicator, an expression, or a
// number, which a range takes two of.
const comparedWithSideSchema = z.discriminatedUnion("valueSource", [
	z.object({
		operator: comparisonOperatorSchema,
		valueSource: z.literal("field"),
		value: variableSchema,
	}),
	z.object({
		operator: comparisonOperatorSchema,
		valueSource: z.literal("expression"),
		value: expressionNodeSchema,
	}),
	z.discriminatedUnion("operator", [
		z.object({
			operator: z.enum(["between", "notBetween"]),
			valueSource: z.literal("value").optional(),
			value: z.tuple([numberSchema, numberSchema]),
		}),
		z.object({
			operator: comparisonOperatorSchema,
			valueSource: z.literal("value").optional(),
			value: numberSchema,
		}),
	]),
]);

// A filled-in condition, parsed to what it exports; issue paths are those of
// the rule (field, lhs, value).
export const strategyRuleSchema = z.intersection(
	comparedSideSchema,
	comparedWithSideSchema,
);

// A query whose every group holds a rule or a group and whose every rule is
// filled in, parsed to what it exports.
export const strategyQuerySchema: z.ZodType<StrategyQuery> = z.object({
	combinator: z.string(),
	not: z.boolean().optional(),
	get rules() {
		return z.array(z.union([strategyQuerySchema, strategyRuleSchema])).min(1);
	},
});
