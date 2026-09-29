import type { CandleInterval } from "@/api/generated/models";

// Parameter inputs by key; an empty input takes the backend default.
export type ParameterValues = Record<string, number | string>;

export interface ScannerIndicatorDraft {
	interval: CandleInterval;
	type: string | null;
	parameters: ParameterValues;
	showInTable: boolean;
	scaleMin: number | string;
	scaleMax: number | string;
	levels: string[];
}
