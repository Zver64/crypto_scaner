import {
	defaultInstrumentAnalysisCriteria,
	type InstrumentAnalysisCriteria,
	validateInstrumentAnalysisCriteria,
} from "./criteria";

export interface InstrumentAnalysisSearch {
	percentile: number;
	period: number;
	unit: "days" | "hours";
}

export function parseInstrumentAnalysisSearch(
	search: Record<string, unknown> | InstrumentAnalysisSearch,
): InstrumentAnalysisSearch {
	const period = search.period;
	const unit = search.unit;
	const percentile = search.percentile;

	if (
		typeof period !== "number" ||
		typeof percentile !== "number" ||
		(unit !== "days" && unit !== "hours") ||
		Object.keys(
			validateInstrumentAnalysisCriteria({ percentile, period, unit }),
		).length > 0
	) {
		return instrumentAnalysisCriteriaToSearch(
			defaultInstrumentAnalysisCriteria,
		);
	}

	return { percentile, period, unit };
}

export function instrumentAnalysisCriteriaToSearch(
	criteria: InstrumentAnalysisCriteria,
): InstrumentAnalysisSearch {
	return {
		percentile: criteria.percentile,
		period: criteria.period,
		unit: criteria.unit,
	};
}

export function instrumentAnalysisCriteriaFromSearch(
	search: InstrumentAnalysisSearch,
): InstrumentAnalysisCriteria {
	return {
		percentile: search.percentile,
		period: search.period,
		unit: search.unit,
	};
}
