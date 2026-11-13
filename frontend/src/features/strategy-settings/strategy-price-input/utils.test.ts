import { describe, expect, it } from "vitest";

import {
	parsePriceSetup,
	priceSetupChanged,
	priceSetupComplete,
	priceSetupExpression,
} from "@/features/strategy-settings/strategy-price-input/utils";

describe("fixed price operands", () => {
	it.each([
		'close("1h")',
		"100.5",
		'close("1h") * 1.05',
		'close("1h") - 2 * atr("1h")',
		'close("1h") + (close("1h") * (5 / 100))',
		'max("1h", 50, high)',
		'min("1h", 50, low)',
		'sma("1d", 50)',
		'Prev(max("1h", 50, high))',
		'Max(close("1h"), open("1h"))',
		'Min(low("1h"), Prev(low("1h")))',
		'close("1h") - Prev(close("1h"))',
		'Abs(close("1h") - open("1h"))',
		'Mod(close("1h"), 2)',
		'Percentile(high("1h"), 50, 100)',
		'Percentile(low("1h"), 50, 0)',
		'Percentile(high("1h"), 50, 100) * 0.99',
	])("restores %s with the existing expression builder", (source) => {
		const setup = parsePriceSetup(source);
		expect(setup.node).toBeDefined();
		expect(setup.custom).toBeUndefined();
		expect(priceSetupComplete(setup)).toBe(true);
		expect(priceSetupExpression(setup)).toBe(source);
	});

	it("serializes edited arithmetic with the shared serializer", () => {
		const setup = parsePriceSetup('close("1h") - 2 * atr("1h", 20)');
		expect(setup.node).toMatchObject({
			kind: "func",
			fn: "subtract",
			args: [
				{ kind: "field", field: 'close("1h")' },
				{ kind: "func", fn: "multiply" },
			],
		});
		expect(priceSetupExpression({ node: setup.node })).toBe(
			'(close("1h") - (2 * atr("1h", 20)))',
		);
	});

	it("serializes a shifted MAX indicator with the shared function serializer", () => {
		const setup = parsePriceSetup('Prev(max("1h", 50, high), 1)');
		expect(setup.node).toMatchObject({
			kind: "func",
			fn: "Prev",
			args: [
				{ kind: "field", field: 'max("1h", 50, high)' },
				{ kind: "value", value: 1 },
			],
		});
		expect(priceSetupExpression({ node: setup.node })).toBe(
			'Prev(max("1h", 50, high), 1)',
		);
	});

	it.each(['Of("BTCUSDT", close("1h"))', 'close("1h") > 0', 'close("1h") +'])(
		"preserves unsupported formula %s",
		(source) => {
			const setup = parsePriceSetup(source);
			expect(setup).toEqual({ custom: source });
			expect(priceSetupExpression(setup)).toBe(source);
		},
	);

	it.each(['close("1h")', 'close("1h") - 2 * atr("1h")'])(
		"clears dirty state when %s is restored without preservation metadata",
		(source) => {
			const initial = parsePriceSetup(source);
			expect(priceSetupChanged(parsePriceSetup('open("1h")'), initial)).toBe(
				true,
			);
			expect(priceSetupChanged({ node: initial.node }, initial)).toBe(false);
		},
	);

	it("tracks disabled, incomplete, and manually edited prices separately", () => {
		const initial = parsePriceSetup('close("1h")');
		const incomplete = { node: { kind: "field" as const, field: "" } };
		expect(priceSetupChanged(incomplete, initial)).toBe(true);
		expect(priceSetupChanged(incomplete, {})).toBe(true);
		expect(priceSetupChanged({}, initial)).toBe(true);
		expect(priceSetupChanged({}, {})).toBe(false);
		const custom = parsePriceSetup('close("1h") +');
		expect(priceSetupChanged({ custom: 'open("1h") +' }, custom)).toBe(true);
		expect(priceSetupChanged({ custom: 'close("1h") +' }, custom)).toBe(false);
	});

	it("blocks incomplete operands and permits removing the price", () => {
		expect(priceSetupComplete({ node: { kind: "field", field: "" } })).toBe(
			false,
		);
		expect(
			priceSetupComplete({ node: { kind: "value", value: Number.NaN } }),
		).toBe(false);
		expect(
			priceSetupComplete({
				node: {
					kind: "func",
					fn: "multiply",
					args: [
						{ kind: "field", field: 'close("1h")' },
						{ kind: "field", field: "" },
					],
				},
			}),
		).toBe(false);
		expect(priceSetupComplete({ custom: "" })).toBe(false);
		expect(priceSetupComplete({})).toBe(true);
		expect(priceSetupExpression({})).toBe("");
	});
});
