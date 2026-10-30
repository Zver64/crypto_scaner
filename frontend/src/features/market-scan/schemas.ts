import { z } from "zod";

import { applicationConfig } from "@/config";
import type { AnalysisUnit } from "@/features/analysis/criteria";
import { percentileSchema, periodSchema } from "@/features/analysis/schemas";
import { marketScanCriteriaConstraints } from "@/features/market-scan/criteria";
import { numberInputSchema } from "@/utils/number-input-schema";

function nonNegativeSchema(label: string) {
	return numberInputSchema(label, (number) =>
		number.min(0, `${label} must be zero or greater`),
	);
}

// The minimum candle range, up to the end of the unit's slider.
function minimumRangeSchema(unit: AnalysisUnit) {
	const { minimum } = marketScanCriteriaConstraints.minimumRangePercent;
	const maximum = applicationConfig.volatility[unit].candleRangeMaximum;
	const range = `Minimum range must be between ${minimum} and ${maximum}`;
	return numberInputSchema("Minimum range", (number) =>
		number.min(minimum, range).max(maximum, range),
	);
}

// The daily and the hourly volatility criterion of a market scan, and its
// minimum market cap.
export const marketScanCriteriaSchema = z.object({
	hourlyMinimumRangePercent: minimumRangeSchema("hours"),
	hourlyPercentile: percentileSchema,
	hourlyPeriod: periodSchema("hours"),
	minimumMarketCapMillions: nonNegativeSchema("Minimum market cap"),
	minimumRangePercent: minimumRangeSchema("days"),
	percentile: percentileSchema,
	period: periodSchema("days"),
});
