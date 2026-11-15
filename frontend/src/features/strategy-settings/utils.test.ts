import { describe, expect, it } from "vitest";

import {
	CandleInterval,
	Direction,
	type StrategyCatalog,
	StrategyCoinSource,
	type StrategyIndicatorParameter,
} from "@/api/generated/models";
import {
	fieldVariable,
	indicatorCall,
	indicatorCallSource,
	replaceFunction,
	variableCall,
	withInterval,
} from "@/features/strategy-settings/expressions";
import type { StrategyQuery } from "@/features/strategy-settings/types";
import {
	defaultVariableCall,
	importedStrategyQuery,
	marketCapErrors,
	marketCapRangeLabel,
	marketCapUsd,
	newStrategyDraft,
	parenthesizeOperands,
	strategyExpression,
	strategyFormDirty,
	strategyInput,
	strategyQuery,
	strategyQueryDropped,
	strategyValidator,
	strategyVariableSets,
	argumentIssuePaths,
	ownIssuePaths,
	validationIssuePaths,
} from "@/features/strategy-settings/utils";

const source = (
	choices: StrategyIndicatorParameter["choices"],
	value = 1,
): StrategyIndicatorParameter => ({
	choices,
	default: value,
	key: "source",
	kind: "source",
	title: "Source",
});

const fields = [
	{ name: "close", title: "Close", value: 1 },
	{ name: "volume", title: "Volume", value: 2 },
];

const period = (value: number): StrategyIndicatorParameter => ({
	default: value,
	key: "period",
	kind: "integer",
	maximum: 100000,
	minimum: 2,
	title: "Time Period",
});

// What the builder reads, shaped like the backend's descriptors.
const strategyCatalog: StrategyCatalog = {
	candle_fields: ["close", "volume"],
	indicator_types: [
		{
			outputs: [
				{ name: "upperband", sparse: false },
				{ name: "lowerband", sparse: false },
			],
			parameters: [
				period(5),
				{
					default: 2,
					key: "deviations_up",
					kind: "real",
					maximum: 3e37,
					minimum: -3e37,
					title: "Deviations up",
				},
				{
					choices: [
						{ title: "SMA", value: 0 },
						{ title: "EMA", value: 1 },
					],
					default: 0,
					key: "ma_type",
					kind: "choice",
					title: "MA Type",
				},
				source(fields),
			],
			title: "Bollinger Bands",
			type: "bbands",
		},
		{
			outputs: [
				{ name: "high", sparse: true },
				{ name: "low", sparse: true },
			],
			parameters: [
				{ ...period(5), key: "left", minimum: 1 },
				{ ...period(5), key: "right", minimum: 1 },
				source([{ title: "High and low", value: 0 }, ...fields], 0),
			],
			title: "Pivot High/Low",
			type: "pivot",
		},
		{
			outputs: [{ name: "rsi", sparse: false }],
			parameters: [period(14), source(fields)],
			title: "Relative Strength Index",
			type: "rsi",
		},
	],
	indicators: [],
	intervals: [CandleInterval["1h"], CandleInterval["1d"]],
	position_variables: [{ count: false, name: "pnl" }],
};

describe("parenthesizeOperands", () => {
	it.each([
		['volume("1h") >= 1.5 * sma("1h")', 'volume("1h") >= (1.5 * sma("1h"))'],
		[
			'close("1h") - open("1h") > Abs(low("1h"))',
			'(close("1h") - open("1h")) > Abs(low("1h"))',
		],
		[
			'close("1h") > -1 && (open("1h") < 2)',
			'close("1h") > -1 && (open("1h") < 2)',
		],
		[
			'CrossesAbove(close("1h") + 1, open("1h")) || !(low("1h") * 2 < 3)',
			'CrossesAbove(close("1h") + 1, open("1h")) || !((low("1h") * 2) < 3)',
		],
		[
			'Prev(close("1h") - open("1h")) > 0',
			'Prev(close("1h") - open("1h")) > 0',
		],
	])("parenthesizes %s", (expression, expected) => {
		expect(parenthesizeOperands(expression)).toBe(expected);
	});
});

describe("replaceFunction", () => {
	const x = { kind: "field", field: 'close("1h")' } as const;
	it.each([
		[
			"ValueWhen",
			"Prev",
			[x, x, { kind: "value", value: 0 }],
			[x, { kind: "value", value: 1 }],
		],
		[
			"Prev",
			"ValueWhen",
			[x, { kind: "value", value: 3 }],
			[x, x, { kind: "value", value: 0 }],
		],
		["add", "subtract", [x, x], [x, { kind: "value", value: 0 }]],
	] as const)(
		"switches %s to %s from its first argument",
		(from, to, args, expected) => {
			expect(
				replaceFunction({ kind: "func", fn: from, args: [...args] }, to),
			).toEqual({
				kind: "func",
				fn: to,
				args: expected,
			});
		},
	);
});

describe("importedStrategyQuery", () => {
	it.each([
		['close("1h") > 1', 'close("1h") > 1'],
		['30 < rsi("1d", 21)', 'rsi("1d", 21) > 30'],
		[
			'(rsi("1h") >= 30 && rsi("1h") <= 70) || close("1d") < 2',
			'(rsi("1h") >= 30 && rsi("1h") <= 70) || close("1d") < 2',
		],
		['close("1h") > sma("1h", 20)', 'close("1h") > sma("1h", 20)'],
		[
			'sma("1h", 20, volume) < sma("1w", 3, trades)',
			'sma("1h", 20, volume) < sma("1w", 3, trades)',
		],
		[
			'Of("BTCUSDT", roc("1h", 12)) < 0\n&& CrossesAbove(close("1h"), Prev(max("1h", 12, high)))',
			'Of("BTCUSDT", roc("1h", 12)) < 0 && CrossesAbove(close("1h"), Prev(max("1h", 12, high)))',
		],
		[
			'Prev((bbands_upperband("1h", 20, 2.5) - bbands_lowerband("1h", 20, 2.5)) / bbands_middleband("1h", 20, 2.5))\n  <= Percentile(high("1h") - low("1h"), 120, 20)\n&& volume("1h") >= 1.5 * sma("1h", 20, volume)',
			'Prev((bbands_upperband("1h", 20, 2.5) - bbands_lowerband("1h", 20, 2.5)) / bbands_middleband("1h", 20, 2.5)) <= Percentile(high("1h") - low("1h"), 120, 20) && volume("1h") >= (1.5 * sma("1h", 20, volume))',
		],
		[
			'close("1h") > bbands_upperband("1h", 20) && pivot_low("1d") < low("1d")',
			'close("1h") > bbands_upperband("1h", 20) && pivot_low("1d") < low("1d")',
		],
		[
			'CrossesAbove(close("1h") + 1, Min(open("1h") * 2, low("1h"))) && Abs(close("1h") - open("1h")) > 1',
			'CrossesAbove(close("1h") + 1, Min(open("1h") * 2, low("1h"))) && Abs(close("1h") - open("1h")) > 1',
		],
		[
			'Prev(close("1h"), BarsSince(pivot_low("1h")) + 5) > Prev(sma("1h", 3, volume), 4)',
			'Prev(close("1h"), BarsSince(pivot_low("1h")) + 5) > Prev(sma("1h", 3, volume), 4)',
		],
		[
			'ValueWhen(pivot_high("1h"), pivot_high("1h"), 0) > ValueWhen(pivot_high("1h", 3, 3), pivot_high("1h", 3, 3), 1)',
			'ValueWhen(pivot_high("1h"), pivot_high("1h"), 0) > ValueWhen(pivot_high("1h", 3, 3), pivot_high("1h", 3, 3), 1)',
		],
		['entry_price < close("1d")', 'entry_price < close("1d")'],
		[
			'bbands_upperband("1h", 20, 0.0000001) > sma("1h", 1000000000000000000000)',
			'bbands_upperband("1h", 20, 0.0000001) > sma("1h", 1000000000000000000000)',
		],
	])("imports %s", (expression, expected) => {
		const query = importedStrategyQuery(expression);
		expect(query && strategyExpression(query)).toBe(expected);
	});

	it.each([
		["with unary minus", '-close("1h") > 1'],
		["without a condition", 'close("1h")'],
	])("rejects an expression %s", (_, expression) => {
		expect(importedStrategyQuery(expression)).toBeUndefined();
	});
});

describe("strategyValidator", () => {
	it("marks incomplete rules and empty groups by id", () => {
		const query = {
			id: "root",
			combinator: "and",
			rules: [
				{ id: "set", field: 'rsi("1h")', operator: ">", value: 1 },
				{
					id: "range",
					field: 'rsi("1h")',
					operator: "between",
					value: [1, Number.NaN],
				},
				{
					id: "coin",
					field: 'rsi("1h")',
					operator: ">",
					valueSource: "expression",
					value: {
						kind: "func",
						fn: "Of",
						args: [
							{ kind: "value", value: "" },
							{ kind: "field", field: 'rsi("1h")' },
						],
					},
				},
				{ id: "empty", combinator: "or", rules: [] },
			],
		} satisfies StrategyQuery;
		const validation = strategyValidator(query);
		expect(validation).toMatchObject({
			root: true,
			set: true,
			range: { valid: false },
			coin: { valid: false },
			empty: false,
		});
		expect(validationIssuePaths(validation.range)).toEqual([["value", 1]]);
		expect(validationIssuePaths(validation.coin)).toEqual([
			["value", "args", 0, "value"],
		]);
	});
});

describe("issue paths of operands", () => {
	const paths = [
		["field"],
		["args"],
		["args", 0, "value"],
		["args", 1, "field"],
	];

	it("keeps an operand's own issues apart from its arguments'", () => {
		expect(ownIssuePaths(paths, false)).toEqual([["field"], ["args"]]);
		expect(ownIssuePaths(paths, true)).toEqual([
			["field"],
			["args"],
			["args", 0, "value"],
		]);
		expect(argumentIssuePaths(paths, 1)).toEqual([["field"]]);
	});
});

describe("strategyExpression", () => {
	it("writes the numbers the inputs hold", () => {
		expect(
			strategyExpression({
				combinator: "and",
				rules: [
					{ field: 'close("1h")', operator: ">", value: "1.50" },
					{ field: 'rsi("1h")', operator: "between", value: ["30.", "70.0"] },
				],
			}),
		).toBe('close("1h") > 1.5 && (rsi("1h") >= 30 && rsi("1h") <= 70)');
	});
});

describe("strategyQuery", () => {
	it("keeps stored comparisons with bare arithmetic operands", () => {
		const expression =
			'close("1d") < Prev(min("1d", 20, low)) || close("1d") < entry_price - 2 * atr("1d", 20)';
		expect(strategyQueryDropped(expression, strategyQuery(expression))).toBe(
			false,
		);
	});

	it("keeps a condition whose call reads another call as its source", () => {
		const expression =
			'pivot_low("1h", 5, 5, rsi("1h")) > 0 && close("1h") > 1';
		const query = strategyQuery(expression);
		expect(strategyExpression(query)).toBe(expression);
		expect(importedStrategyQuery(expression)).toBeDefined();
	});

	it("reads a compared call as the condition's field", () => {
		expect(strategyQuery('rsi("1h") < sma("1d", 20)').rules).toEqual([
			{
				field: 'rsi("1h")',
				operator: "<",
				value: 'sma("1d", 20)',
				valueSource: "field",
			},
		]);
	});
});

describe("strategyVariableSets", () => {
	const { entry, exit } = strategyVariableSets(strategyCatalog);

	it("offers every output of every indicator type, calls on a period", () => {
		expect(exit.variables.map(({ name }) => name)).toEqual([
			"close",
			"volume",
			"bbands_upperband",
			"bbands_lowerband",
			"pivot_high",
			"pivot_low",
			"rsi",
			"pnl",
		]);
		expect(entry.variables.map(({ name }) => name)).not.toContain("pnl");
		expect(
			exit.variables.map((variable) => variableCall(variable, "1d")),
		).toEqual([
			'close("1d")',
			'volume("1d")',
			'bbands_upperband("1d")',
			'bbands_lowerband("1d")',
			'pivot_high("1d")',
			'pivot_low("1d")',
			'rsi("1d")',
			"pnl",
		]);
		expect(exit.fields.map(({ label }) => label)).toEqual([
			"Candle",
			"Indicators",
			"Position",
		]);
		expect(entry.fields.map(({ label }) => label)).not.toContain("Position");
		expect(entry.defaultField).toBe('close("1h")');
	});

	it("finds the variable and period of a call", () => {
		expect(fieldVariable('bbands_upperband("1d", 20)')).toEqual({
			interval: "1d",
			name: "bbands_upperband",
		});
		expect(fieldVariable("pnl")).toEqual({ name: "pnl" });
		expect(withInterval('sma("1h", 9, rsi("1h", 21))', "1w")).toBe(
			'sma("1w", 9, rsi("1w", 21))',
		);
	});

	it("offers another call on the same period", () => {
		expect(defaultVariableCall(exit, 'close("1d")')).toBe('volume("1d")');
	});
});

describe("indicatorCall", () => {
	const { exit } = strategyVariableSets(strategyCatalog);

	it.each([
		'rsi("1h")',
		'rsi("1h", 21, volume)',
		'bbands_upperband("1h", 20, 2.5)',
		'bbands_lowerband("1d", 5, 2, 1)',
		'pivot_low("1d", 5, 5, rsi("1d", 21))',
		'pivot_high("1h", 3, 5, close)',
		'rsi("1h", 9, bbands_upperband("1h", 20, 2.5, 1, volume))',
		// A source whose indicator is still to be chosen.
		'rsi("1h", 14, ())',
	])("spells %s back as it reads it", (field) => {
		const call = indicatorCall(field, exit.variables);
		expect(call && indicatorCallSource(call)).toBe(field);
	});

	it("reads the parameters a call leaves out at their defaults", () => {
		const call = indicatorCall('pivot_low("1d", 3)', exit.variables);
		expect(call?.args).toEqual(["3", "5", ""]);
	});

	it("drops trailing defaults, the source a nested call too", () => {
		const call = indicatorCall(
			'bbands_upperband("1h", 20, 2.5, 1, rsi("1h", 21))',
			exit.variables,
		);
		if (!call) throw new Error("not read");
		expect(
			indicatorCallSource({ ...call, args: ["20", "2", "0", "close"] }),
		).toBe('bbands_upperband("1h", 20)');
		expect(
			indicatorCallSource({
				...call,
				args: call.args.map((arg) =>
					typeof arg === "string" ? arg : { ...arg, args: ["14", "close"] },
				),
			}),
		).toBe('bbands_upperband("1h", 20, 2.5, 1, rsi("1h"))');
	});

	it.each([
		// A sparse output or a source of a source is not a source.
		'rsi("1h", 14, pivot_low("1h"))',
		'rsi("1h", 14, rsi("1h", 9, rsi("1h")))',
		'rsi("1h", 14, close, 1)',
		'close("1h")',
	])("does not read %s", (field) => {
		expect(indicatorCall(field, exit.variables)).toBeUndefined();
	});

	it("opens the RSI divergence rule", () => {
		const expression = [
			'pivot_low("1h", 5, 5, rsi("1h")) > 0',
			'pivot_low("1h", 5, 5, rsi("1h")) > ValueWhen(pivot_low("1h", 5, 5, rsi("1h")), pivot_low("1h", 5, 5, rsi("1h")), 1)',
			'Prev(low("1h"), 5) < ValueWhen(pivot_low("1h", 5, 5, rsi("1h")), Prev(low("1h"), 5), 1)',
			'Prev(BarsSince(pivot_low("1h", 5, 5, rsi("1h"))), 1) >= 5',
			'Prev(BarsSince(pivot_low("1h", 5, 5, rsi("1h"))), 1) <= 60',
		].join("\n&& ");
		const query = strategyQuery(expression);
		expect(strategyQueryDropped(expression, query)).toBe(false);
		expect(strategyExpression(query)).toBe(expression.replaceAll("\n", " "));
	});
});

describe("marketCapUsd", () => {
	it.each([
		[150, 150_000_000],
		[1.1, 1_100_000],
		[0.0005, 500],
		["150.", 150_000_000],
		["", null],
		["-", Number.NaN],
	] as const)("converts %s million", (millions, usd) => {
		expect(marketCapUsd(millions)).toBe(usd);
	});
});

describe("marketCapErrors", () => {
	it.each([
		[null, null, {}],
		[10, 20, {}],
		[0, null, { minimum: "Must be a positive amount" }],
		[null, Number.NaN, { maximum: "Must be a positive amount" }],
		[20, 10, { maximum: "Must not be below the minimum" }],
	])("checks %s to %s", (minimum, maximum, errors) => {
		expect(marketCapErrors(minimum, maximum)).toEqual(errors);
	});
});

describe("marketCapRangeLabel", () => {
	it.each([
		[10_000_000, 1_500_000_000, "$10M – $1.5B"],
		[1_000_000_000, null, "≥ $1B"],
		[null, 500_000_000, "≤ $500M"],
		[null, null, undefined],
	])("labels %s to %s", (minimum, maximum, label) => {
		expect(
			marketCapRangeLabel({
				max_market_cap_usd: maximum,
				min_market_cap_usd: minimum,
			}),
		).toBe(label);
	});
});

describe("strategyFormDirty", () => {
	const { values } = newStrategyDraft("strategy");
	const initial = { ...values, query: strategyQuery('close("1h") > 1') };

	it("compares rules by the expression they save", () => {
		// The builder adds ids to the rules it edits.
		const edited = {
			...initial.query,
			rules: initial.query.rules.map((rule) => ({ ...rule, id: "rule" })),
		};
		expect(
			strategyFormDirty(initial, { ...initial, query: edited }, false),
		).toBe(false);
		expect(
			strategyFormDirty(
				initial,
				{ ...initial, query: strategyQuery('close("1h") > 2') },
				false,
			),
		).toBe(true);
	});

	it("ignores the price fields of a signal", () => {
		const takeProfit = { custom: 'close("1h") * 1.1' };
		expect(strategyFormDirty(initial, { ...initial, takeProfit }, true)).toBe(
			false,
		);
		expect(strategyFormDirty(initial, { ...initial, takeProfit }, false)).toBe(
			true,
		);
	});
});

describe("strategyInput", () => {
	const { values } = newStrategyDraft("signal");
	const named = {
		...values,
		name: "Signal",
		query: strategyQuery('close("1h") > 1'),
		coins: ["SOLUSDT", "BTCUSDT"],
		direction: Direction.long,
		targetRatio: "3",
		window: "12",
	};

	it("sends the selects' choices as numbers", () => {
		expect(strategyInput(named, true)).toMatchObject({
			target_ratio: 3,
			window: 12,
		});
	});

	it("sends no target ratio or window for a simple signal", () => {
		expect(
			strategyInput({ ...named, direction: Direction.simple }, true),
		).toMatchObject({ target_ratio: null, window: null });
	});

	it("sends the coins, or none for a signal of the favorites or all coins", () => {
		expect(strategyInput(named, true)).toMatchObject({
			coin_source: StrategyCoinSource.CoinSourceSelected,
			coins: ["BTCUSDT", "SOLUSDT"],
		});
		for (const coinSource of [
			StrategyCoinSource.CoinSourceFavorites,
			StrategyCoinSource.CoinSourceAll,
		]) {
			expect(strategyInput({ ...named, coinSource }, true)).toMatchObject({
				coin_source: coinSource,
				coins: [],
			});
			// Only a signal evaluates the favorites or all coins.
			expect(strategyInput({ ...named, coinSource }, false)).toMatchObject({
				coin_source: StrategyCoinSource.CoinSourceSelected,
				coins: ["BTCUSDT", "SOLUSDT"],
			});
		}
	});

	it("is undefined while the form cannot be saved", () => {
		expect(strategyInput({ ...named, name: " " }, true)).toBeUndefined();
		expect(strategyInput({ ...named, coins: [] }, true)).toBeUndefined();
		expect(strategyInput({ ...named, minMarketCap: 0 }, true)).toBeUndefined();
		expect(strategyInput({ ...named, window: "7" }, true)).toBeUndefined();
	});
});
