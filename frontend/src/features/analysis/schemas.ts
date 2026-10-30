import { z } from "zod";

import { applicationConfig } from "@/config";
import {
	type AnalysisUnit,
	analysisCriteriaConstraints,
} from "@/features/analysis/criteria";
import { numberInputSchema } from "@/utils/number-input-schema";

const { period } = analysisCriteriaConstraints;

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

const percentilePresets: readonly number[] =
	applicationConfig.volatility.percentilePresets;

// Picked on a slider of its presets, so it is one of them.
export const percentileSchema = numberInputSchema(
	"Range percentile",
	(number) =>
		number.refine(
			(value) => percentilePresets.includes(value),
			`Range percentile must be one of ${percentilePresets.join(", ")}`,
		),
);

// The daily and the hourly volatility criterion of Top Market Cap and
// Favorites.
export const volatilitySettingsSchema = z.object({
	hourlyPercentile: percentileSchema,
	hourlyPeriod: periodSchema("hours"),
	percentile: percentileSchema,
	period: periodSchema("days"),
});
