import {
	type AnalysisCriteria,
	type AnalysisDraft,
	defaultMarketScanCriteria,
	validateAnalysisCriteria,
} from "../market-scan/criteria";

export type InstrumentAnalysisCriteria = AnalysisCriteria;
export type InstrumentAnalysisDraft = AnalysisDraft;

export const defaultInstrumentAnalysisCriteria: InstrumentAnalysisCriteria = {
	percentile: defaultMarketScanCriteria.percentile,
	periodDays: defaultMarketScanCriteria.periodDays,
};

export const validateInstrumentAnalysisCriteria = validateAnalysisCriteria;
