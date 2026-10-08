import { describe, expect, it } from "vitest";
import {
	parsePriceSetup,
	priceSetupChanged,
	priceSetupComplete,
	priceSetupExpression,
} from "@/features/strategy-settings/strategy-price-input/utils";

describe("fixed price operands", () => {
	it.each([
		"h_close",
		"100.5",
		"h_close * 1.05",
		"h_close - 2 * h_atr_14",
		"h_close + (h_close * (5 / 100))",
		"h_max_50_high",
		"h_min_50_low",
		"h_sma_50",
		"prev(h_max_50_high)",
		"max(h_close, h_open)",
		"min(h_low, prev(h_low))",
		"h_close - prev(h_close)",
		"abs(h_close - h_open)",
		"mod(h_close, 2)",
		"percentile(h_high, 50, 100)",
		"percentile(h_low, 50, 0)",
		"percentile(h_high, 50, 100) * 0.99",
	])("restores %s with the existing expression builder", (source) => {
		const setup = parsePriceSetup(source);
		expect(setup.node).toBeDefined();
		expect(setup.custom).toBeUndefined();
		expect(priceSetupComplete(setup)).toBe(true);
		expect(priceSetupExpression(setup)).toBe(source);
	});

	it("serializes edited arithmetic with the shared serializer", () => {
		const setup = parsePriceSetup("h_close - 2 * h_atr_14");
		expect(setup.node).toMatchObject({
			kind: "func",
			fn: "subtract",
			args: [
				{ kind: "field", field: "h_close" },
				{ kind: "func", fn: "multiply" },
			],
		});
		expect(priceSetupExpression({ node: setup.node })).toBe(
			"(h_close - (2 * h_atr_14))",
		);
	});

	it("serializes a shifted MAX indicator with the shared function serializer", () => {
		const setup = parsePriceSetup("prev(h_max_50_high, 1)");
		expect(setup.node).toMatchObject({
			kind: "func",
			fn: "prev",
			args: [
				{ kind: "field", field: "h_max_50_high" },
				{ kind: "value", value: 1 },
			],
		});
		expect(priceSetupExpression({ node: setup.node })).toBe(
			"prev(h_max_50_high, 1)",
		);
	});

	it.each([
		'of("BTCUSDT", h_close)',
		"h_close > 0",
		"h_close +",
	])("preserves unsupported formula %s", (source) => {
		const setup = parsePriceSetup(source);
		expect(setup).toEqual({ custom: source });
		expect(priceSetupExpression(setup)).toBe(source);
	});

	it.each([
		"h_close",
		"h_close - 2 * h_atr_14",
	])("clears dirty state when %s is restored without preservation metadata", (source) => {
		const initial = parsePriceSetup(source);
		expect(priceSetupChanged(parsePriceSetup("h_open"), initial)).toBe(true);
		expect(priceSetupChanged({ node: initial.node }, initial)).toBe(false);
	});

	it("tracks disabled, incomplete, and manually edited prices separately", () => {
		const initial = parsePriceSetup("h_close");
		const incomplete = { node: { kind: "field" as const, field: "" } };
		expect(priceSetupChanged(incomplete, initial)).toBe(true);
		expect(priceSetupChanged(incomplete, {})).toBe(true);
		expect(priceSetupChanged({}, initial)).toBe(true);
		expect(priceSetupChanged({}, {})).toBe(false);
		const custom = parsePriceSetup("h_close +");
		expect(priceSetupChanged({ custom: "h_open +" }, custom)).toBe(true);
		expect(priceSetupChanged({ custom: "h_close +" }, custom)).toBe(false);
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
						{ kind: "field", field: "h_close" },
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
