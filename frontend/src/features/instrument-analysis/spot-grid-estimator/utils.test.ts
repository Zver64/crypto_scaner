import { describe, expect, it } from "vitest";
import {
	calculateSpotGridInput,
	spotGridEstimateValues,
} from "@/features/instrument-analysis/spot-grid-estimator/utils";

const validInput = {
	lowerPrice: "100",
	upperPrice: "120",
	gridCount: "2",
	investment: "220",
};

const zeroValues = {
	averageEntryPrice: "0 USDT",
	profitPerStep: "0 USDT",
	profitPerStepPercent: "0%",
};

describe("calculateSpotGridInput", () => {
	it.each(
		Object.keys(validInput) as (keyof typeof validInput)[],
	)("clears the estimate for partial input missing %s", (field) => {
		const calculation = calculateSpotGridInput({ ...validInput, [field]: "" });

		expect(calculation).toBeNull();
		expect(spotGridEstimateValues(calculation?.estimate ?? null)).toEqual(
			zeroValues,
		);
	});

	it("clears a valid estimate after cleared or invalid input", () => {
		const validCalculation = calculateSpotGridInput(validInput);

		expect(validCalculation?.estimate).not.toBeNull();

		const clearedCalculation = calculateSpotGridInput({
			...validInput,
			investment: "",
		});
		expect(clearedCalculation).toBeNull();
		expect(
			spotGridEstimateValues(clearedCalculation?.estimate ?? null),
		).toEqual(zeroValues);

		const invalidCalculation = calculateSpotGridInput({
			...validInput,
			upperPrice: "100",
		});
		expect(invalidCalculation?.estimate).toBeNull();
		expect(invalidCalculation?.error).toContain("Upper price");
		expect(
			spotGridEstimateValues(invalidCalculation?.estimate ?? null),
		).toEqual(zeroValues);
	});
});
