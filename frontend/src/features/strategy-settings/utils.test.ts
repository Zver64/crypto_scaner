import { describe, expect, it } from "vitest";

import {
	CandleInterval,
	Direction,
	StrategyCoinSource,
} from "@/api/generated/models";
import {
	fieldVariable,
	variableCall,
	withInterval,
} from "@/features/strategy-settings/expressions";
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
	strategyVariableSets,
} from "@/features/strategy-settings/utils";

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
			'sma("1h", 20, volume) < sma("1w", 3, trade_count)',
			'sma("1h", 20, volume) < sma("1w", 3, trade_count)',
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
			'Prev(close("1h"), pivot_low_bars("1h") - 1) > Prev(sma("1h", 3, volume), 4)',
			'Prev(close("1h"), pivot_low_bars("1h") - 1) > Prev(sma("1h", 3, volume), 4)',
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

describe("strategyQuery", () => {
	it("keeps stored comparisons with bare arithmetic operands", () => {
		const expression =
			'close("1d") < Prev(min("1d", 20, low)) || close("1d") < entry_price - 2 * atr("1d", 20)';
		expect(strategyQueryDropped(expression, strategyQuery(expression))).toBe(
			false,
		);
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
	const catalog = {
		candle_fields: ["close"],
		indicators: [
			{
				count_outputs: [],
				outputs: ["upperband", "lowerband"],
				parameters: ["20"],
				title: "bbands-20",
				type: "bbands",
			},
		],
		intervals: [CandleInterval["1h"], CandleInterval["1d"]],
		position_variables: [{ count: false, name: "pnl" }],
	};
	const read = {
		count_outputs: [],
		outputs: ["real"],
		parameters: ["21"],
		title: "rsi-21",
		type: "rsi",
	};

	it("offers the configured and the read indicators once each, calls on a period", () => {
		const { entry, exit } = strategyVariableSets(catalog, [
			read,
			catalog.indicators[0],
		]);
		expect(exit.variables.map(({ key }) => key)).toEqual([
			"close()",
			"bbands_upperband(20)",
			"bbands_lowerband(20)",
			"rsi(21)",
			"pnl",
		]);
		expect(entry.variables.map(({ key }) => key)).not.toContain("pnl");
		expect(
			exit.variables.map((variable) => variableCall(variable, "1d")),
		).toEqual([
			'close("1d")',
			'bbands_upperband("1d", 20)',
			'bbands_lowerband("1d", 20)',
			'rsi("1d", 21)',
			"pnl",
		]);
		expect(entry.fields.map(({ name }) => name)).toEqual(['close("1h")']);
	});

	it("finds the variable and period of a call", () => {
		expect(fieldVariable('bbands_upperband("1d", 20)')).toEqual({
			interval: "1d",
			key: "bbands_upperband(20)",
		});
		expect(fieldVariable('rsi("1h")')).toEqual({
			interval: "1h",
			key: "rsi()",
		});
		expect(fieldVariable("pnl")).toEqual({ key: "pnl" });
		expect(withInterval('sma("1h", 20, volume)', "1w")).toBe(
			'sma("1w", 20, volume)',
		);
	});

	it("offers another call on the same period", () => {
		const { exit } = strategyVariableSets(catalog, []);
		expect(defaultVariableCall(exit, 'close("1d")')).toBe(
			'bbands_upperband("1d", 20)',
		);
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
