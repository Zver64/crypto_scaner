import { describe, expect, it } from "vitest";

import {
	priceNodeSchema,
	strategyQuerySchema,
	strategyRuleSchema,
} from "@/features/strategy-settings/schemas";

const issuePaths = (result: {
	error?: { issues: { path: PropertyKey[] }[] };
}) => result.error?.issues.map(({ path }) => path);

describe("strategyRuleSchema", () => {
	it("parses numbers as their inputs hold them", () => {
		expect(
			strategyRuleSchema.parse({
				field: 'rsi("1h")',
				operator: "between",
				value: ["1.50", "2."],
			}),
		).toEqual({ field: 'rsi("1h")', operator: "between", value: [1.5, 2] });
		expect(
			strategyRuleSchema.parse({
				field: 'rsi("1h")',
				operator: ">",
				valueSource: "expression",
				value: {
					kind: "func",
					fn: "multiply",
					args: [
						{ kind: "field", field: 'sma("1h")' },
						{ kind: "value", value: "1.0" },
					],
				},
			}).value,
		).toEqual({
			kind: "func",
			fn: "multiply",
			args: [
				{ kind: "field", field: 'sma("1h")' },
				{ kind: "value", value: 1 },
			],
		});
	});

	it.each([
		[
			"an unchosen field and an empty number",
			{ field: "", operator: ">", value: "" },
			[["field"], ["value"]],
		],
		[
			"a range bound that is not a number",
			{ field: 'rsi("1h")', operator: "between", value: [1, "-"] },
			[["value", 1]],
		],
		[
			"a single number for a range",
			{ field: 'rsi("1h")', operator: "between", value: 1 },
			[["value"]],
		],
		[
			"an unchosen coin and indicator",
			{
				field: 'rsi("1h")',
				operator: ">",
				valueSource: "expression",
				value: {
					kind: "func",
					fn: "Of",
					args: [
						{ kind: "value", value: "" },
						{ kind: "field", field: "" },
					],
				},
			},
			[
				["value", "args", 0, "value"],
				["value", "args", 1, "field"],
			],
		],
		[
			"a wrong argument count beside an incomplete argument",
			{
				field: 'rsi("1h")',
				lhs: {
					kind: "func",
					fn: "Abs",
					args: [
						{ kind: "field", field: "" },
						{ kind: "value", value: 1 },
					],
				},
				operator: ">",
				value: 1,
			},
			[
				["lhs", "args", 0, "field"],
				["lhs", "args"],
			],
		],
	])("points at %s", (_, rule, paths) => {
		expect(issuePaths(strategyRuleSchema.safeParse(rule))).toEqual(paths);
	});

	it("accepts an expression whose field is not chosen", () => {
		expect(
			strategyRuleSchema.safeParse({
				field: "",
				lhs: { kind: "field", field: 'rsi("1h")' },
				operator: ">",
				value: 1,
			}).success,
		).toBe(true);
	});
});

describe("strategyQuerySchema", () => {
	const rule = { field: 'rsi("1h")', operator: ">", value: 1 };

	it("needs a rule or group in every group", () => {
		expect(
			strategyQuerySchema.safeParse({ combinator: "and", rules: [rule] })
				.success,
		).toBe(true);
		expect(
			strategyQuerySchema.safeParse({ combinator: "and", rules: [] }).success,
		).toBe(false);
		expect(
			strategyQuerySchema.safeParse({
				combinator: "and",
				rules: [rule, { combinator: "or", rules: [] }],
			}).success,
		).toBe(false);
	});
});

describe("priceNodeSchema", () => {
	it.each([
		["an unchosen indicator", { kind: "field", field: "" }],
		["an empty number", { kind: "value", value: "" }],
		[
			"an incomplete argument",
			{
				kind: "func",
				fn: "multiply",
				args: [
					{ kind: "field", field: 'close("1h")' },
					{ kind: "field", field: "" },
				],
			},
		],
		[
			"another coin",
			{
				kind: "func",
				fn: "Of",
				args: [
					{ kind: "value", value: "BTCUSDT" },
					{ kind: "field", field: 'close("1h")' },
				],
			},
		],
	])("rejects %s", (_, node) => {
		expect(priceNodeSchema.safeParse(node).success).toBe(false);
	});
});
