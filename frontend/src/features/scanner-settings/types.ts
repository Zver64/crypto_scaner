import type { CandleInterval } from "@/api/generated/models";

// Parameter inputs by key; an empty input takes the backend default.
export type ParameterValues = Record<string, number | string>;

// Chosen periods; a period is chosen when it has an entry.
export type PeriodChoices = Partial<
	Record<CandleInterval, { showInTable: boolean }>
>;

export interface ScannerIndicatorDraft {
	periods: PeriodChoices;
	type: string | null;
	parameters: ParameterValues;
	scaleMin: number | string;
	scaleMax: number | string;
	levels: string[];
}
