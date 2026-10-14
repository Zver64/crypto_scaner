import { z } from "zod";

import { percentileSchema, periodSchema } from "@/features/analysis/schemas";
import { numberInputSchema } from "@/utils/number-input-schema";

function nonNegativeSchema(label: string) {
	return numberInputSchema(label, (number) =>
		number.min(0, `${label} must be zero or greater`),
	);
}

const minimumRangeSchema = nonNegativeSchema("Minimum range");

// The daily and the hourly volatility criterion of a market scan, and its
// minimum market cap.
export const marketScanCriteriaSchema = z.object({
	hourlyMinimumRangePercent: minimumRangeSchema,
	hourlyPercentile: percentileSchema,
	hourlyPeriod: periodSchema("hours"),
	minimumMarketCapMillions: nonNegativeSchema("Minimum market cap"),
	minimumRangePercent: minimumRangeSchema,
	percentile: percentileSchema,
	period: periodSchema("days"),
});
