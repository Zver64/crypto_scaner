import { FormApi } from "@tanstack/react-form";
import { describe, expect, it } from "vitest";

import type { GridLimitsResponse } from "@/api/generated/models";
import { gridCalculatorValues } from "@/features/instrument-analysis/grid-estimator/defaults";
import { gridCountLimitErrors } from "@/features/instrument-analysis/grid-estimator/grid-count-limit";
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
	usdm: null,
};

// The calculator form with its grid count check and a spot grid of
// 55000–70000 in 100 arithmetic grids, which Binance accepts up to 107.
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
		investment: "1000",
		lowerPrice: "55000",
		upperPrice: "70000",
	};
	const defaultValues: GridCalculatorFormValues = {
		...values,
		markets: {
			...values.markets,
			spot: {
				...values.markets.spot,
				...committed,
				committed,
				gridType: "arithmetic",
			},
		},
	};
	const form = new FormApi({
		defaultValues,
		validators: {
			onChange: ({ value }) => gridCountLimitErrors(value, limits),
		},
	});
	form.mount();
	return form;
}

const gridCountErrors = (form: ReturnType<typeof calculatorForm>) =>
	form.getFieldMeta("markets.spot.gridCount")?.errors ?? [];

describe("grid count limit check", () => {
	it("checks the typed grid count against the Binance range", () => {
		const form = calculatorForm();
		form.setFieldValue("markets.spot.gridCount", "108");
		expect(gridCountErrors(form)).toEqual(["Maximum is 107 grids"]);
		form.setFieldValue("markets.spot.gridCount", "107");
		expect(gridCountErrors(form)).toEqual([]);
		form.setFieldValue("markets.spot.gridCount", "1");
		expect(gridCountErrors(form)).toEqual(["Minimum is 2 grids"]);
	});

	it("follows the maximum of the committed grid", () => {
		const form = calculatorForm();
		form.setFieldValue("markets.spot.gridCount", "115");
		expect(gridCountErrors(form)).toEqual(["Maximum is 107 grids"]);
		form.setFieldValue("markets.spot.gridType", "geometric");
		expect(gridCountErrors(form)).toEqual([]);
		form.setFieldValue("markets.spot.committed", {
			...form.state.values.markets.spot.committed,
			lowerPrice: "62500",
			upperPrice: "62600",
		});
		expect(gridCountErrors(form)).toEqual([
			"Price range is too narrow for a grid",
		]);
	});
});
