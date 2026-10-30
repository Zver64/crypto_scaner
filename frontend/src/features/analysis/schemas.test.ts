import { describe, expect, it } from "vitest";
import { z } from "zod";

import { defaultVolatilitySettings } from "@/features/analysis/criteria";
import { volatilitySettingsSchema } from "@/features/analysis/schemas";

const defaultSettings = defaultVolatilitySettings;

function fieldErrors(values: z.input<typeof volatilitySettingsSchema>) {
	const result = volatilitySettingsSchema.safeParse(values);
	return result.success ? {} : z.flattenError(result.error).fieldErrors;
}

describe("volatility settings", () => {
	it("accepts supported day and hour boundaries", () => {
		expect(fieldErrors(defaultSettings)).toEqual({});
		expect(
			fieldErrors({
				hourlyPercentile: 95,
				hourlyPeriod: 2000,
				percentile: 75,
				period: 1,
			}),
		).toEqual({});
	});

	it("requires numbers", () => {
		expect(
			fieldErrors({ ...defaultSettings, percentile: "", period: "30" }),
		).toEqual({
			percentile: ["Range percentile is required"],
			period: ["Analysis period must be a number"],
		});
	});

	it("rejects values outside the analysis constraints", () => {
		expect(
			fieldErrors({
				hourlyPercentile: 85,
				hourlyPeriod: 2001,
				percentile: 101,
				period: 0,
			}),
		).toEqual({
			hourlyPercentile: ["Range percentile must be one of 75, 80, 90, 95"],
			hourlyPeriod: [
				"Analysis period must be a whole number between 1 and 2000 hours",
			],
			percentile: ["Range percentile must be one of 75, 80, 90, 95"],
			period: [
				"Analysis period must be a whole number between 1 and 2000 days",
			],
		});
	});
});
