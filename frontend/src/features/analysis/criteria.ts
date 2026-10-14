export type AnalysisUnit = "days" | "hours";

export interface AnalysisCriteria {
	percentile: number;
	period: number;
	unit: AnalysisUnit;
}

export const defaultAnalysisCriteria: AnalysisCriteria = {
	percentile: 80,
	period: 30,
	unit: "days",
};

export const analysisCriteriaConstraints = {
	percentile: { maximum: 100, minimum: 0 },
	// Mirrors the backend market.SyncDepth.
	period: { maximum: 2000, minimum: 1 },
} as const;

export function defaultPeriodForUnit(unit: AnalysisUnit): number {
	return unit === "hours" ? 60 : 30;
}
