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
	period: defaultMarketScanCriteria.period,
	unit: defaultMarketScanCriteria.unit,
};

export const validateInstrumentAnalysisCriteria = validateAnalysisCriteria;
