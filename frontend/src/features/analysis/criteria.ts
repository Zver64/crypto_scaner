import { applicationConfig } from "@/config";
import type { VolatilitySettings } from "@/features/analysis/volatility-settings-form/types";

export type AnalysisUnit = "days" | "hours";

export interface AnalysisCriteria {
	percentile: number;
	period: number;
	unit: AnalysisUnit;
}

const { days, hours } = applicationConfig.volatility;

export const defaultVolatilitySettings: VolatilitySettings = {
	hourlyPercentile: hours.defaultPercentile,
	hourlyPeriod: hours.defaultPeriod,
	percentile: days.defaultPercentile,
	period: days.defaultPeriod,
};

export const analysisCriteriaConstraints = {
	// Mirrors the backend market.SyncDepth.
	period: { maximum: 2000, minimum: 1 },
} as const;
