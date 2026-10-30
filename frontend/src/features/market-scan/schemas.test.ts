import { expect, it } from "vitest";
import { z } from "zod";

import { defaultMarketScanCriteria } from "@/features/market-scan/pipeline";
import { marketScanCriteriaSchema } from "@/features/market-scan/schemas";

function fieldErrors(values: z.input<typeof marketScanCriteriaSchema>) {
	const result = marketScanCriteriaSchema.safeParse(values);
	return result.success ? {} : z.flattenError(result.error).fieldErrors;
}

it("accepts zero minimum ranges and market cap", () => {
	expect(
		fieldErrors({
			...defaultMarketScanCriteria,
			hourlyMinimumRangePercent: 0,
			minimumMarketCapMillions: 0,
			minimumRangePercent: 0,
		}),
	).toEqual({});
});

it("validates both mandatory volatility instances independently", () => {
	expect(
		fieldErrors({
			period: 2001,
			percentile: 101,
			minimumRangePercent: -1,
			hourlyPeriod: 2001,
			hourlyPercentile: -1,
			hourlyMinimumRangePercent: "",
			minimumMarketCapMillions: "",
		}),
	).toEqual({
		period: ["Analysis period must be a whole number between 1 and 2000 days"],
		percentile: ["Range percentile must be one of 75, 80, 90, 95"],
		minimumRangePercent: ["Minimum range must be between 0 and 10"],
		hourlyPeriod: [
			"Analysis period must be a whole number between 1 and 2000 hours",
		],
		hourlyPercentile: ["Range percentile must be one of 75, 80, 90, 95"],
		hourlyMinimumRangePercent: ["Minimum range is required"],
		minimumMarketCapMillions: ["Minimum market cap is required"],
	});
});

it.each([
	"period",
	"percentile",
	"minimumRangePercent",
	"hourlyPeriod",
	"hourlyPercentile",
	"hourlyMinimumRangePercent",
	"minimumMarketCapMillions",
] as const)("rejects missing and non-numeric %s", (field) => {
	for (const value of ["2", Number.NaN, Number.POSITIVE_INFINITY]) {
		expect(
			fieldErrors({ ...defaultMarketScanCriteria, [field]: value })[field]?.[0],
		).toMatch(/ must be a number$/);
	}
});
