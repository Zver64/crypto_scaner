import {
	defaultInstrumentAnalysisCriteria,
	type InstrumentAnalysisCriteria,
	validateInstrumentAnalysisCriteria,
} from "./criteria";

export interface InstrumentAnalysisSearch {
	percentile: number;
	period_days: number;
}

export function parseInstrumentAnalysisSearch(
	search: Record<string, unknown> | InstrumentAnalysisSearch,
): InstrumentAnalysisSearch {
	const periodDays = search.period_days;
	const percentile = search.percentile;

	if (
		typeof periodDays !== "number" ||
		typeof percentile !== "number" ||
		Object.keys(validateInstrumentAnalysisCriteria({ percentile, periodDays }))
			.length > 0
	) {
		return instrumentAnalysisCriteriaToSearch(
			defaultInstrumentAnalysisCriteria,
		);
	}

	return { percentile, period_days: periodDays };
}

export function instrumentAnalysisCriteriaToSearch(
	criteria: InstrumentAnalysisCriteria,
): InstrumentAnalysisSearch {
	return {
		percentile: criteria.percentile,
		period_days: criteria.periodDays,
	};
}

export function instrumentAnalysisCriteriaFromSearch(
	search: InstrumentAnalysisSearch,
): InstrumentAnalysisCriteria {
	return {
		percentile: search.percentile,
		periodDays: search.period_days,
	};
}
