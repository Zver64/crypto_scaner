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
				hourlyPercentile: 100,
				hourlyPeriod: 2000,
				percentile: 0,
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
				hourlyPercentile: 80.5,
				hourlyPeriod: 2001,
				percentile: 101,
				period: 0,
			}),
		).toEqual({
			hourlyPercentile: ["Range percentile must be a whole number"],
			hourlyPeriod: [
				"Analysis period must be a whole number between 1 and 2000 hours",
			],
			percentile: ["Range percentile must be between 0 and 100"],
			period: [
				"Analysis period must be a whole number between 1 and 2000 days",
			],
		});
	});
});
