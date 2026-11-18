import { FormApi } from "@tanstack/react-form";
import { describe, expect, it } from "vitest";

import type { GridLimitsResponse } from "@/api/generated/models";
import { gridCalculatorValues } from "@/features/instrument-analysis/grid-estimator/defaults";
import { investmentMinimumErrors } from "@/features/instrument-analysis/grid-estimator/investment-minimum";
import type { GridCalculatorFormValues } from "@/features/instrument-analysis/grid-estimator/types";

const limits: GridLimitsResponse = {
	coinm: null,
	spot: {
		ask_multiplier_up: 5,
		average_price: 62530,
		bid_multiplier_down: 0.2,
		max_price: 0,
		min_notional: 5,
		min_price: 0.01,
		min_qty: 0.00001,
		step_size: 0.00001,
		tick_size: 0.01,
	},
	symbol: "BTCUSDT",
	usdm: {
		mark_price: 62432,
		min_notional: 50,
		min_qty: 0.001,
		step_size: 0.001,
		symbol: "BTCUSDT",
		tick_size: 0.1,
	},
};

// The calculator form with its investment check and a USDT-M long grid of
// 55000–70000 in 100 arithmetic grids at 10×, whose minimum is 1032.5 USDT.
function calculatorForm() {
	const values = gridCalculatorValues(
		{
			builtFor: "test",
			close: 62530,
			dailyVolatilityPercent: undefined,
			gridLimits: limits,
			hourlyVolatilityPercent: 1,
		},
		null,
		new Set(),
	);
	const committed = {
		gridCount: "100",
		investment: "1040",
		lowerPrice: "55000",
		upperPrice: "70000",
	};
	const defaultValues: GridCalculatorFormValues = {
		...values,
		markets: {
			...values.markets,
			usdm: {
				...values.markets.usdm,
				...committed,
				committed,
				direction: "long",
				gridType: "arithmetic",
				leverage: 10,
			},
		},
	};
	const form = new FormApi({
		defaultValues,
		validators: {
			onChange: ({ value }) => investmentMinimumErrors(value, limits, "BTC"),
		},
	});
	form.mount();
	return form;
}

const investmentErrors = (form: ReturnType<typeof calculatorForm>) =>
	form.getFieldMeta("markets.usdm.investment")?.errors ?? [];

describe("investment minimum check", () => {
	it("follows the minimum of the committed grid, direction, and leverage", () => {
		const form = calculatorForm();
		form.setFieldValue("markets.usdm.leverage", 1);
		expect(investmentErrors(form)).toEqual(["Minimum is 8134.1 USDT"]);
		form.setFieldValue("markets.usdm.leverage", 10);
		expect(investmentErrors(form)).toEqual([]);
		form.setFieldValue("markets.usdm.direction", "short");
		expect(investmentErrors(form)).toEqual(["Minimum is 1047.4 USDT"]);
		form.setFieldValue("markets.usdm.committed", {
			...form.state.values.markets.usdm.committed,
			gridCount: "50",
		});
		expect(investmentErrors(form)).toEqual([]);
	});

	it("checks the typed investment", () => {
		const form = calculatorForm();
		form.setFieldValue("markets.usdm.investment", "1000");
		expect(investmentErrors(form)).toEqual(["Minimum is 1032.5 USDT"]);
	});
});
