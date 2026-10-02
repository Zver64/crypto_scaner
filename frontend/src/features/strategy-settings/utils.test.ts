import { describe, expect, it } from "vitest";
import {
	importedStrategyQuery,
	parenthesizeOperands,
	strategyExpression,
} from "@/features/strategy-settings/utils";

describe("parenthesizeOperands", () => {
	it.each([
		["h_volume >= 1.5 * h_sma", "h_volume >= (1.5 * h_sma)"],
		["h_close - h_open > abs(h_low)", "(h_close - h_open) > abs(h_low)"],
		["h_close > -1 && (h_open < 2)", "h_close > -1 && (h_open < 2)"],
		[
			"crosses_above(h_close + 1, h_open) || !(h_low * 2 < 3)",
			"crosses_above(h_close + 1, h_open) || !((h_low * 2) < 3)",
		],
		["prev(h_close - h_open) > 0", "prev(h_close - h_open) > 0"],
	])("parenthesizes %s", (expression, expected) => {
		expect(parenthesizeOperands(expression)).toBe(expected);
	});
});

describe("importedStrategyQuery", () => {
	it.each([
		["h_close > 1", "h_close > 1"],
		[
			"(h_rsi >= 30 && h_rsi <= 70) || d_close < 2",
			"(h_rsi >= 30 && h_rsi <= 70) || d_close < 2",
		],
		[
			'of("BTCUSDT", h_roc) < 0\n&& crosses_above(h_close, prev(h_max))',
			'of("BTCUSDT", h_roc) < 0 && crosses_above(h_close, prev(h_max))',
		],
		[
			"prev((h_upper - h_lower) / h_middle)\n  <= percentile(h_upper - h_lower, 120, 20)\n&& h_volume >= 1.5 * h_sma",
			"prev((h_upper - h_lower) / h_middle) <= percentile(h_upper - h_lower, 120, 20) && h_volume >= (1.5 * h_sma)",
		],
		[
			"crosses_above(h_close + 1, min(h_open * 2, h_low)) && abs(h_close - h_open) > 1",
			"crosses_above(h_close + 1, min(h_open * 2, h_low)) && abs(h_close - h_open) > 1",
		],
	])("imports %s", (expression, expected) => {
		const query = importedStrategyQuery(expression);
		expect(query && strategyExpression(query)).toBe(expected);
	});

	it.each([
		["with unary minus", "-h_close > 1"],
		["without a condition", "h_close"],
	])("rejects an expression %s", (_, expression) => {
		expect(importedStrategyQuery(expression)).toBeUndefined();
	});
});
