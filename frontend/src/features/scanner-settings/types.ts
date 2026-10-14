import type { CandleInterval } from "@/api/generated/models";

// Parameter inputs by key, as the inputs emit them: a number input's number or
// string, a choice select's string. An empty input takes the backend default.
export type ParameterValues = Record<string, number | string | null>;

// Where a chosen period shows the indicator.
export interface PeriodChoice {
	showInTable: boolean;
	showInChart: boolean;
}

// Chosen periods; a period is chosen when it has an entry.
export type PeriodChoices = Partial<Record<CandleInterval, PeriodChoice>>;

// Pane scale inputs; an empty bound is automatic.
export interface ScaleDraft {
	scaleMin: number | string;
	scaleMax: number | string;
	levels: string[];
}

export interface ScannerIndicatorDraft extends ScaleDraft {
	periods: PeriodChoices;
	type: string | null;
	parameters: ParameterValues;
}
