import { z } from "zod";

import {
	type AnalysisUnit,
	analysisCriteriaConstraints,
} from "@/features/analysis/criteria";
import { numberInputSchema } from "@/utils/number-input-schema";

const { percentile, period } = analysisCriteriaConstraints;

// The analysis period in candles of the unit.
export function periodSchema(unit: AnalysisUnit) {
	const range = `Analysis period must be a whole number between ${period.minimum} and ${period.maximum} ${unit}`;
	return numberInputSchema("Analysis period", (number) =>
		number
			.int("Analysis period must be a whole number")
			.min(period.minimum, range)
			.max(period.maximum, range),
	);
}

const percentileRange = `Range percentile must be between ${percentile.minimum} and ${percentile.maximum}`;

export const percentileSchema = numberInputSchema(
	"Range percentile",
	(number) =>
		number
			.int("Range percentile must be a whole number")
			.min(percentile.minimum, percentileRange)
			.max(percentile.maximum, percentileRange),
);

// The daily and the hourly volatility criterion of Top Market Cap and
// Favorites.
export const volatilitySettingsSchema = z.object({
	hourlyPercentile: percentileSchema,
	hourlyPeriod: periodSchema("hours"),
	percentile: percentileSchema,
	period: periodSchema("days"),
});
